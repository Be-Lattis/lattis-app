package bridge

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/lalamefine/lattis/internal/doc"
	"github.com/lalamefine/lattis/internal/remote/auth"
	"github.com/lalamefine/lattis/internal/remote/workbook"
	"github.com/lalamefine/lattis/internal/trace"
)

// Classeur ouvert depuis un Serveur Lattis : son fichier local n'est qu'un
// cache (<cache>/partages/<hôte>/<identifiant>.lattis). L'interface ne le
// montre jamais : titre du classeur sur le serveur, « en ligne sur
// <serveur> », adresse <serveur>/w/<id> (DocState.Remote).

// remoteTitle : titre d'un classeur du serveur sans titre connu (serveur
// d'avant la migration 0007, ancienne entrée des récents).
const remoteTitle = "Classeur partagé"

// inShareCache dit si path est dans le cache des classeurs du serveur.
func inShareCache(path string) bool {
	if path == "" {
		return false
	}
	dir, err := shareCacheDir()
	if err != nil {
		return false
	}
	rel, err := filepath.Rel(dir, path)
	return err == nil && rel != "." && !filepath.IsAbs(rel) && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

// remoteDoc décrit le classeur du serveur dont d est le cache (nil : un
// fichier local, publié ou non).
func remoteDoc(d *doc.Document) *workbook.State {
	if d == nil || d.Share == nil || d.Share.Info() == nil || !inShareCache(d.Path()) {
		return nil
	}
	st := d.Share.State()
	if strings.TrimSpace(st.Title) == "" {
		st.Title = remoteTitle
	}
	return &st
}

// keepCollabQueue enregistre, avant sa fermeture, le cache d'un classeur
// collaboratif du serveur qui a des modifications pas encore envoyées
// (file d'envoi, conflits) : hors ligne, elles partiront à la prochaine
// ouverture une fois le serveur revenu, même si l'utilisateur a répondu
// « Ne pas enregistrer » (en ligne, elles seraient déjà parties ; le
// cache n'est pas un fichier de l'utilisateur). Décision 0014. Une erreur
// rendue : le cache n'a pas pu être enregistré ; l'appelant garde alors le
// répertoire de travail (récupération au prochain lancement) au lieu de
// le fermer, et le dit (errQueueKept).
func keepCollabQueue(d *doc.Document) error {
	r := remoteDoc(d)
	if r == nil || r.Mode != workbook.Collaborative || d.ReadOnly() {
		return nil
	}
	ctx := context.Background()
	pending, conflicts, err := workbook.Unsent(ctx, d.DB().Read())
	if err != nil || pending+conflicts == 0 && !d.Share.LayoutUnsent() {
		return nil
	}
	if err := saveDoc(ctx, d, ""); err != nil {
		trace.Logf(1, trace.Doc, "cache de « %s » non enregistré (%d étape(s) en attente) : %v", r.Title, pending, err)
		return fmt.Errorf("La copie locale de « %s » n'a pas pu être enregistrée : ses modifications pas encore envoyées sont gardées et proposées en récupération (Fichier › Ouvrir, ou au prochain lancement de Lattis)", r.Title)
	}
	return nil
}

// closeKeeping ferme le classeur d en gardant sa file d'envoi
// (keepCollabQueue) ; si elle n'a pas pu être enregistrée, le répertoire de
// travail reste (récupération) et l'erreur est rendue.
func closeKeeping(d *doc.Document) error {
	if err := keepCollabQueue(d); err != nil {
		d.Release()
		return err
	}
	d.Close()
	return nil
}

// serverLabel : nom du serveur, sinon son hôte.
func serverLabel(server, name string) string {
	if strings.TrimSpace(name) != "" {
		return name
	}
	if h := auth.Host(server); h != "" {
		return h
	}
	return server
}

// remoteSavedNotice : message de la barre d'état après Ctrl+S sur un
// classeur du serveur. Le fichier enregistré n'est que la copie locale :
// le message dit ce qui est vraiment arrivé au serveur.
func remoteSavedNotice(r *workbook.State) string {
	where := serverLabel(r.Server, r.ServerName)
	switch {
	case r.Mode == workbook.Collaborative && !r.Online:
		return fmt.Sprintf("Hors ligne : « %s » enregistré sur ce poste ; vos modifications partiront vers %s à la reconnexion", r.Title, where)
	case r.Mode == workbook.Collaborative && r.Pending > 0:
		s := "s"
		if r.Pending == 1 {
			s = ""
		}
		return fmt.Sprintf("Enregistré sur %s : %s (%d modification%s en cours d'envoi)", where, r.Title, r.Pending, s)
	case r.Mode == workbook.Collaborative:
		return fmt.Sprintf("Enregistré sur %s : %s", where, r.Title)
	case r.CanDesign:
		return fmt.Sprintf("Copie locale de « %s » enregistrée ; Fichier › Republier pour publier vos changements sur %s", r.Title, where)
	}
	return fmt.Sprintf("Copie locale de « %s » enregistrée (classeur publié sur %s, en lecture seule)", r.Title, where)
}

// fileTitle : nom de fichier proposé pour un titre (Enregistrer sous,
// export) ; les caractères interdits par Windows deviennent des espaces.
func fileTitle(title string) string {
	t := strings.Map(func(r rune) rune {
		if r < 32 || strings.ContainsRune(`<>:"/\|?*`, r) {
			return ' '
		}
		return r
	}, title)
	t = strings.Trim(strings.Join(strings.Fields(t), " "), ". ")
	if t == "" {
		return "Classeur"
	}
	return t
}

// docBaseName : nom (sans extension) proposé pour le classeur d : titre du
// classeur du serveur, nom de son fichier, sinon « Classeur ».
func docBaseName(d *doc.Document) string {
	if r := remoteDoc(d); r != nil {
		return fileTitle(r.Title)
	}
	if d.Path() == "" {
		return "Classeur"
	}
	base := filepath.Base(d.Path())
	return strings.TrimSuffix(base, filepath.Ext(base))
}

// docFolder : dossier proposé par les boîtes Enregistrer sous et Exporter
// (celui du classeur ; jamais le cache d'un classeur du serveur).
func docFolder(d *doc.Document) string {
	if inShareCache(d.Path()) {
		return ""
	}
	return docDir(d.Path())
}
