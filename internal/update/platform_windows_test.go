package update

import (
	"os"
	"path/filepath"
	"testing"
)

// Un exécutable non signé (le binaire de test) : signature refusée, et
// l'application qui ne l'est pas n'exige rien de ses mises à jour.
func TestAuthenticodeUnsigned(t *testing.T) {
	exe, err := os.Executable()
	if err != nil {
		t.Skip(err)
	}
	if VerifySignature(exe) == nil {
		t.Skip("binaire de test signé : rien à vérifier ici")
	}
	if SelfSigned(exe) {
		t.Error("binaire non signé vu signé")
	}
	junk := filepath.Join(t.TempDir(), "junk.exe")
	os.WriteFile(junk, []byte("MZ pas un exécutable"), 0o644)
	if VerifySignature(junk) == nil {
		t.Error("fichier quelconque accepté")
	}
}

// Le binaire de test n'est dans aucun dossier d'installation : portable.
func TestDetectKindPortable(t *testing.T) {
	exe, _ := os.Executable()
	if k := DetectKind(exe); k != KindPortable && k != KindMSIX {
		t.Errorf("binaire de test : %s", k)
	}
}
