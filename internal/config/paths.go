package config

import (
	"os"
	"path/filepath"
	"runtime"
)

// machineDir est le dossier de la configuration posée par l'administrateur.
func machineDir() string {
	switch runtime.GOOS {
	case "windows":
		pd := os.Getenv("ProgramData")
		if pd == "" {
			pd = `C:\ProgramData`
		}
		return filepath.Join(pd, "Lattis")
	case "darwin":
		return "/Library/Application Support/Lattis"
	}
	return "/etc/lattis"
}

// userFile est la configuration de l'utilisateur (réglages de l'appli).
func userFile() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		return ""
	}
	return filepath.Join(dir, "Lattis", "config.json")
}

// cacheFile est le cache de la configuration distante.
func cacheFile() string {
	dir, err := os.UserCacheDir()
	if err != nil {
		return ""
	}
	return filepath.Join(dir, "Lattis", "remote-config.json")
}

// localSources donne les couches locales, de la moins à la plus
// prioritaire, sans le défaut ni la configuration distante.
func localSources(setFlags []string, environ func() []string) []Source {
	dir := machineDir()
	out := []Source{&FileSource{Path: filepath.Join(dir, "config.json"), R: RankMachine, AdminFile: true}}
	out = append(out, dirSources(filepath.Join(dir, "config.d"), RankMachine, true)...)
	if f := userFile(); f != "" {
		out = append(out, &FileSource{Path: f, R: RankUser})
	}
	out = append(out, registrySources()...)
	out = append(out, &EnvSource{Environ: environ}, &FlagSource{Pairs: setFlags})
	out = append(out, &FileSource{Path: filepath.Join(dir, "policy.json"), R: RankPolicy, AdminFile: true})
	return append(out, registryPolicySources()...)
}
