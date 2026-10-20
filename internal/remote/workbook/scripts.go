package workbook

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"

	"github.com/lalamefine/lattis/internal/store"
)

// Scripts d'un classeur collaboratif (audit du mode en ligne, constat 20).
// Publiés avec le classeur, ils se partagent ensuite comme la mise en page :
// un propriétaire ou un concepteur qui enregistre un script le publie
// (lattis.publish_script, nouvelle version ; « *meta »), les autres postes
// reçoivent la dernière version de chaque script. Les autres membres ne
// modifient pas les scripts (canvas.Model.SaveScript, LayoutLock). Un script
// reçu ne s'exécute qu'après l'accord de l'utilisateur sur cette version
// (internal/trust, par empreinte du source).
//
// Base : empreinte de chaque script au dernier accord avec le serveur
// (_lattis_info, scriptsBaseKey). Changé ici seulement : publié (concepteur) ;
// changé sur le serveur seulement : appliqué ; changé des deux côtés : la
// version du concepteur l'emporte chez lui, celle du serveur chez les autres.
// Un script supprimé ici n'est pas retiré du serveur (pas de fonction pour
// cela) : il revient au prochain rechargement complet.

// scriptsBaseKey : empreintes des scripts au dernier accord avec le serveur.
const scriptsBaseKey = "scripts_base"

// ScriptSync : scripts publiés (Sent) et reçus (Received) par SyncScripts.
type ScriptSync struct {
	Sent, Received []string
}

func scriptSum(src string) string {
	h := sha256.Sum256([]byte(src))
	return hex.EncodeToString(h[:])
}

// localScriptMap lit les scripts du classeur : nom → source.
func localScriptMap(ctx context.Context, q interface {
	QueryContext(ctx context.Context, q string, args ...any) (*sql.Rows, error)
}) (map[string]string, error) {
	rs, err := q.QueryContext(ctx, `SELECT name, source FROM _lattis_scripts`)
	if err != nil {
		return nil, err
	}
	defer rs.Close()
	out := map[string]string{}
	for rs.Next() {
		var n, s string
		if err := rs.Scan(&n, &s); err != nil {
			return nil, err
		}
		out[n] = s
	}
	return out, rs.Err()
}

func readScriptsBase(ctx context.Context, q interface {
	QueryRowContext(ctx context.Context, q string, args ...any) *sql.Row
}) (map[string]string, bool, error) {
	var raw string
	err := q.QueryRowContext(ctx, `SELECT value FROM _lattis_info WHERE key = ?`, scriptsBaseKey).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		return map[string]string{}, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	out := map[string]string{}
	return out, true, json.Unmarshal([]byte(raw), &out)
}

// SyncScripts accorde les scripts du classeur ouvert avec ceux du serveur
// (voir plus haut). Sans base (classeur ouvert avant cette synchronisation),
// la base est la version du serveur : rien n'est publié, les différences
// sont reçues.
func SyncScripts(ctx context.Context, srv Server, db *store.Store, info *Info) (ScriptSync, error) {
	var res ScriptSync
	if info == nil || info.Mode != Collaborative {
		return res, nil
	}
	var remote []Script
	if err := srv.Delta(ctx, func(r Reader) error {
		var err error
		remote, err = r.Scripts(ctx, info.Schema)
		return err
	}); err != nil {
		return res, err
	}
	theirs := map[string]string{}
	for _, s := range remote {
		theirs[s.Name] = s.Source
	}
	local, err := localScriptMap(ctx, db.Read())
	if err != nil {
		return res, err
	}
	base, known, err := readScriptsBase(ctx, db.Read())
	if err != nil {
		return res, err
	}
	if !known {
		for n, s := range theirs {
			base[n] = scriptSum(s)
		}
	}
	design := info.CanDesign()
	publish := map[string]string{}
	apply := map[string]string{}
	for n, src := range local {
		sum := scriptSum(src)
		srvSrc, onServer := theirs[n]
		switch {
		case onServer && scriptSum(srvSrc) == sum:
			// Identiques.
		case sum != base[n] && design:
			publish[n] = src // changé ici par un concepteur
		case onServer:
			apply[n] = srvSrc // changé sur le serveur (ou ici sans droit)
		}
	}
	for n, src := range theirs {
		if _, ok := local[n]; !ok {
			apply[n] = src // nouveau sur le serveur
		}
	}
	if len(publish) > 0 {
		err := srv.Write(ctx, func(w Writer) error {
			for n, src := range publish {
				if _, err := w.PublishScript(ctx, info.WorkbookID, n, src); err != nil {
					return err
				}
			}
			return nil
		})
		if err != nil {
			return res, err
		}
		for n, src := range publish {
			theirs[n] = src
			res.Sent = append(res.Sent, n)
		}
	}
	next := map[string]string{}
	for n, src := range theirs {
		next[n] = scriptSum(src)
	}
	raw, _ := json.Marshal(next)
	err = db.Write(ctx, func(tx *sql.Tx) error {
		for n, src := range apply {
			if _, err := tx.ExecContext(ctx, `INSERT INTO _lattis_scripts (id, name, source) VALUES ((SELECT coalesce(max(id), 0) + 1 FROM _lattis_scripts), ?, ?)
				ON CONFLICT (name) DO UPDATE SET source = excluded.source`, n, src); err != nil {
				return err
			}
			res.Received = append(res.Received, n)
		}
		_, err := tx.ExecContext(ctx, `INSERT INTO _lattis_info (key, value) VALUES (?, ?) ON CONFLICT (key) DO UPDATE SET value = excluded.value`, scriptsBaseKey, string(raw))
		return err
	})
	return res, err
}
