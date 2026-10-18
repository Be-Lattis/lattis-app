package config

import (
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/google/uuid"
)

// DeviceID est l'identifiant stable de ce poste pour l'utilisateur
// (dossier de configuration de l'utilisateur, Lattis/device-id), créé au
// premier appel. Un Serveur Lattis le reçoit sur les connexions d'écriture
// (SET lattis.device, contrat client §6) : deux postes du même utilisateur
// sont deux auteurs, chacun reçoit ce que l'autre a écrit. Sans dossier de
// configuration utilisable, un identifiant propre à ce lancement.
var DeviceID = sync.OnceValue(func() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		return uuid.NewString()
	}
	return deviceIDAt(filepath.Join(dir, "Lattis", "device-id"))
})

// deviceIDAt lit l'identifiant du poste dans path, ou l'y crée.
func deviceIDAt(path string) string {
	if b, err := os.ReadFile(path); err == nil {
		if id, err := uuid.Parse(strings.TrimSpace(string(b))); err == nil {
			return id.String()
		}
	}
	id := uuid.NewString()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err == nil {
		_ = os.WriteFile(path, []byte(id+"\n"), 0o600)
	}
	return id
}
