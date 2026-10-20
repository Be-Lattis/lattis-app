package bridge

import (
	"context"
	"github.com/lalamefine/lattis/internal/msg"
	"io"
	"os"
	"path/filepath"
)

// Édition web : un classeur local vit dans le répertoire de la session, sur
// le serveur (audit du mode en ligne, constat 22). Ctrl+S l'y enregistre ;
// seul le premier enregistrement le télécharge. Fichier › Télécharger une
// copie l'enregistre puis en télécharge le fichier, à tout moment.

// errNotWeb : Télécharger n'existe que dans l'édition web (au bureau, le
// fichier est déjà sur le poste).
var errNotWeb = msg.New("doc.downloadWebOnly", nil)

// Download enregistre le classeur (dans la session) puis en fait télécharger
// le fichier par le navigateur.
func (s *DocumentService) Download(ctx context.Context) (DocState, error) {
	if !s.sys().Platform().Web {
		return DocState{}, errNotWeb
	}
	d, err := s.current()
	if err != nil {
		return DocState{}, err
	}
	if remoteDoc(d) != nil {
		return DocState{}, msg.New("doc.downloadServer", nil)
	}
	first := d.Path() == ""
	if dirty, _ := d.Dirty(ctx); dirty || first {
		st, err := s.Save(ctx, false)
		if err != nil || st.Dirty || st.Conflict != nil || first {
			// Premier enregistrement : il vient d'être téléchargé.
			return st, err
		}
	}
	src := d.Path()
	if _, err := s.sys().SaveFile(FileDialog{Title: "Télécharger une copie", Filename: filepath.Base(src), Ext: ".lattis"}, func(dst string) error {
		if samePath(dst, src) {
			return nil
		}
		return copyTo(src, dst)
	}); err != nil {
		return DocState{}, err
	}
	st, err := s.State(ctx)
	st.Notice = s.say("doc.downloaded", msg.Args{"file": filepath.Base(src)})
	return st, err
}

// copyTo copie le fichier src vers dst.
func copyTo(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}
