package update

import (
	"bytes"
	"crypto/ed25519"
	"encoding/base64"
	"errors"
	"strings"
)

// Signature du manifeste : latest.json est signé en Ed25519 par la CI
// (cmd/lattis-updsign, secret UPDATE_SIGNING_KEY) et la signature publiée à
// côté, dans latest.json.sig (base64 standard des 64 octets, sur une ligne).
// Le message signé est le contenu exact de latest.json. Les empreintes
// SHA-256 des installeurs y sont : une release modifiée sur GitHub (compte
// ou jeton volé) ne peut plus faire installer un autre fichier.
//
// Clés publiques acceptées : publishedKeys, dans le code de l'application.
// Liste vide : aucune signature exigée (versions publiées jusqu'ici, et tant
// que la clé n'est pas créée) ; dès qu'une clé y figure, une version sans
// signature valide n'est plus installée. Mode d'emploi :
// docs/release/signature.md § 4.
const SignatureName = ManifestName + ".sig"

// publishedKeys : clés publiques Ed25519 (base64 des 32 octets) dont une
// signature de latest.json est acceptée. Plusieurs : changement de clé
// (l'ancienne reste le temps qu'une version signée de la nouvelle soit
// installée partout).
var publishedKeys = []string{}

// maxSignature : taille maximale de latest.json.sig.
const maxSignature = 1 << 10

// ManifestKeys : les clés publiques acceptées (publishedKeys) ; une clé
// mal écrite fait échouer les essais (TestPublishedKeys).
func ManifestKeys() []ed25519.PublicKey {
	var out []ed25519.PublicKey
	for _, s := range publishedKeys {
		if k, err := ParsePublicKey(s); err == nil {
			out = append(out, k)
		}
	}
	return out
}

// ParsePublicKey lit une clé publique Ed25519 en base64 (32 octets).
func ParsePublicKey(s string) (ed25519.PublicKey, error) {
	b, err := base64.StdEncoding.DecodeString(strings.TrimSpace(s))
	if err != nil {
		return nil, err
	}
	if len(b) != ed25519.PublicKeySize {
		return nil, errors.New("clé publique Ed25519 : 32 octets attendus")
	}
	return ed25519.PublicKey(b), nil
}

// ParsePrivateKey lit une clé privée Ed25519 en base64 : la graine de 32
// octets (lattis-updsign keygen) ou la clé complète de 64.
func ParsePrivateKey(s string) (ed25519.PrivateKey, error) {
	b, err := base64.StdEncoding.DecodeString(strings.TrimSpace(s))
	if err != nil {
		return nil, err
	}
	switch len(b) {
	case ed25519.SeedSize:
		return ed25519.NewKeyFromSeed(b), nil
	case ed25519.PrivateKeySize:
		k := ed25519.PrivateKey(b)
		// Clé complète : sa moitié publique doit être celle de la graine.
		if !bytes.Equal(ed25519.NewKeyFromSeed(k.Seed()), k) {
			return nil, errors.New("clé privée Ed25519 incohérente")
		}
		return k, nil
	}
	return nil, errors.New("clé privée Ed25519 : 32 ou 64 octets attendus")
}

// SignManifest signe le contenu de latest.json : le texte de latest.json.sig.
func SignManifest(key ed25519.PrivateKey, manifest []byte) []byte {
	return []byte(base64.StdEncoding.EncodeToString(ed25519.Sign(key, manifest)) + "\n")
}

// VerifyManifest : sig (latest.json.sig) est une signature de manifest par
// l'une des clés.
func VerifyManifest(manifest, sig []byte, keys []ed25519.PublicKey) bool {
	raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(sig)))
	if err != nil || len(raw) != ed25519.SignatureSize {
		return false
	}
	for _, k := range keys {
		if len(k) == ed25519.PublicKeySize && ed25519.Verify(k, manifest, raw) {
			return true
		}
	}
	return false
}
