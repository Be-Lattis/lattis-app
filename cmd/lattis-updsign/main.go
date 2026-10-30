// Commande lattis-updsign : clé et signature Ed25519 du manifeste des
// versions (latest.json), vérifiée par la mise à jour automatique avant
// tout téléchargement (internal/update/sign.go, docs/release/signature.md § 4).
//
//	go run ./cmd/lattis-updsign keygen [fichier]   crée la clé (lattis-update.key par défaut)
//	go run ./cmd/lattis-updsign sign latest.json    écrit latest.json.sig (clé : UPDATE_SIGNING_KEY)
//	go run ./cmd/lattis-updsign verify latest.json  vérifie latest.json.sig avec les clés de l'application
//
// sign sans clé : rien n'est signé (avertissement) tant que l'application
// n'exige aucune signature ; échec dès qu'elle en exige une, pour ne jamais
// publier une version que les installations ne pourraient pas installer.
package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"os"

	"github.com/lalamefine/lattis/internal/update"
)

func main() {
	if len(os.Args) < 2 {
		usage()
	}
	var err error
	switch os.Args[1] {
	case "keygen":
		out := "lattis-update.key"
		if len(os.Args) > 2 {
			out = os.Args[2]
		}
		err = keygen(out)
	case "sign":
		if len(os.Args) != 3 {
			usage()
		}
		err = sign(os.Args[2])
	case "verify":
		if len(os.Args) != 3 {
			usage()
		}
		err = verify(os.Args[2])
	default:
		usage()
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "lattis-updsign :", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage : lattis-updsign keygen [fichier] | sign latest.json | verify latest.json")
	os.Exit(2)
}

// keygen écrit la graine de la clé privée (base64) dans out, jamais
// par-dessus un fichier existant, et affiche la clé publique.
func keygen(out string) error {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return err
	}
	f, err := os.OpenFile(out, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintln(f, base64.StdEncoding.EncodeToString(priv.Seed())); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	fmt.Printf("Clé privée : %s (secret GitHub UPDATE_SIGNING_KEY ; à garder hors du dépôt)\n", out)
	fmt.Printf("Clé publique, à ajouter à publishedKeys (internal/update/sign.go) :\n%s\n", base64.StdEncoding.EncodeToString(pub))
	return nil
}

func sign(path string) error {
	embedded := update.ManifestKeys()
	raw := os.Getenv("UPDATE_SIGNING_KEY")
	if raw == "" {
		if len(embedded) > 0 {
			return fmt.Errorf("UPDATE_SIGNING_KEY absent, alors que l'application exige un latest.json signé : les installations refuseraient cette version")
		}
		fmt.Println("::warning::Pas de clé UPDATE_SIGNING_KEY : latest.json n'est pas signé (docs/release/signature.md § 4).")
		return nil
	}
	key, err := update.ParsePrivateKey(raw)
	if err != nil {
		return fmt.Errorf("UPDATE_SIGNING_KEY : %w", err)
	}
	pub := key.Public().(ed25519.PublicKey)
	known := false
	for _, k := range embedded {
		known = known || k.Equal(pub)
	}
	if len(embedded) > 0 && !known {
		return fmt.Errorf("la clé UPDATE_SIGNING_KEY (publique : %s) n'est pas dans publishedKeys : les installations refuseraient cette signature", base64.StdEncoding.EncodeToString(pub))
	}
	if !known {
		fmt.Printf("::warning::latest.json signé, mais la clé publique %s n'est pas encore dans publishedKeys (internal/update/sign.go) : aucune installation ne la vérifie.\n", base64.StdEncoding.EncodeToString(pub))
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	sig := update.SignManifest(key, b)
	if !update.VerifyManifest(b, sig, []ed25519.PublicKey{pub}) {
		return fmt.Errorf("signature produite invalide")
	}
	if err := os.WriteFile(path+".sig", sig, 0o644); err != nil {
		return err
	}
	fmt.Printf("%s.sig écrit\n", path)
	return nil
}

func verify(path string) error {
	keys := update.ManifestKeys()
	if len(keys) == 0 {
		return fmt.Errorf("aucune clé dans publishedKeys (internal/update/sign.go)")
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	sig, err := os.ReadFile(path + ".sig")
	if err != nil {
		return err
	}
	if !update.VerifyManifest(b, sig, keys) {
		return fmt.Errorf("%s.sig : signature invalide", path)
	}
	fmt.Println("signature valide")
	return nil
}
