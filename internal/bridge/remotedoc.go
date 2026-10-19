package bridge

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/lalamefine/lattis/internal/doc"
	"github.com/lalamefine/lattis/internal/remote/auth"
	"github.com/lalamefine/lattis/internal/remote/workbook"
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
