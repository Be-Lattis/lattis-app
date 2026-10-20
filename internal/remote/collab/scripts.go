package collab

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"

	"github.com/lalamefine/lattis/internal/remote/workbook"
)

// scriptsSignature résume les scripts du classeur (nom et source) : la
// synchronisation des scripts ne relit le serveur que si elle change, ou à
// la connexion et sur « *meta ».
func (e *Engine) scriptsSignature(ctx context.Context) (string, error) {
	rs, err := e.db.Read().QueryContext(ctx, `SELECT name, source FROM _lattis_scripts`)
	if err != nil {
		return "", err
	}
	defer rs.Close()
	var parts []string
	for rs.Next() {
		var n, s string
		if err := rs.Scan(&n, &s); err != nil {
			return "", err
		}
		parts = append(parts, n+"\x00"+s)
	}
	sort.Strings(parts)
	h := sha256.Sum256([]byte(strings.Join(parts, "\x01")))
	return hex.EncodeToString(h[:]), rs.Err()
}

// SyncScripts accorde les scripts du classeur avec le serveur
// (workbook.SyncScripts) : publiés par un concepteur qui les a changés,
// reçus des autres. Sans changement local, rien n'est relu sauf force
// (connexion, « *meta »). Un refus du serveur (droits retirés) laisse les
// scripts sur le poste, sans erreur.
func (e *Engine) SyncScripts(ctx context.Context, force bool) (Report, error) {
	e.run.Lock()
	defer e.run.Unlock()
	var rep Report
	info, srv := e.active()
	if info == nil || !e.link.Online() || e.link.Gone() != "" {
		return rep, nil
	}
	sig, err := e.scriptsSignature(ctx)
	if err != nil || (sig == e.scripts && !force) {
		return rep, err
	}
	res, err := workbook.SyncScripts(ctx, srv, e.db, info)
	if err != nil {
		if _, ok := workbook.Refusal(err); !ok {
			return rep, err
		}
	}
	if len(res.Received) > 0 {
		rep.Layout = true
		sort.Strings(res.Received)
		rep.notice(scriptsNotice(res.Received))
	}
	if e.scripts, err = e.scriptsSignature(ctx); err != nil {
		return rep, err
	}
	return rep, nil
}

// scriptsNotice : message des scripts reçus d'un concepteur.
func scriptsNotice(names []string) string {
	if len(names) == 1 {
		return fmt.Sprintf("Script « %s » mis à jour par un concepteur du classeur (votre accord sera demandé avant sa prochaine exécution)", names[0])
	}
	return fmt.Sprintf("%d scripts mis à jour par un concepteur du classeur (votre accord sera demandé avant leur prochaine exécution)", len(names))
}
