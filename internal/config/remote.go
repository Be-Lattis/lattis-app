package config

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"github.com/lalamefine/lattis/internal/msg"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Configuration distante : un document JSON servi en https à l'adresse
// configUrl, posée par un administrateur (stratégie, Registre HKLM ou
// fichier machine). Lattis ne l'attend jamais au démarrage : il lit la
// dernière réponse en cache, puis la rafraîchit en arrière-plan.
//
// Le document peut être signé : JWS compact (EdDSA, Ed25519), vérifié avec
// remote.publicKey. Une clé posée rend la signature obligatoire. Seul un
// document signé avec une clé posée par un administrateur peut porter une
// stratégie, dans sa clé « policy » :
//
//	{ "connections": {...}, "policy": { "features": { "mcp": { "enabled": false } } } }

// maxRemoteSize borne la taille d'un document distant.
const maxRemoteSize = 1 << 20

// RemoteDoc est un document distant lu et vérifié.
type RemoteDoc struct {
	Config Tree
	// Policy : partie « policy », gardée seulement si le document est signé.
	Policy Tree
	Signed bool
}

// ParsePublicKey lit une clé publique Ed25519 : PEM (PUBLIC KEY) ou 32
// octets en base64.
func ParsePublicKey(s string) (ed25519.PublicKey, error) {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "-----BEGIN") {
		blk, _ := pem.Decode([]byte(s))
		if blk == nil {
			return nil, msg.New("config.publicKeyPem", nil)
		}
		k, err := x509.ParsePKIXPublicKey(blk.Bytes)
		if err != nil {
			return nil, msg.Wrap(err, "config.publicKeyUnreadable", msg.Args{"err": err})
		}
		pk, ok := k.(ed25519.PublicKey)
		if !ok {
			return nil, msg.New("config.publicKeyEd25519", nil)
		}
		return pk, nil
	}
	for _, enc := range []*base64.Encoding{base64.StdEncoding, base64.RawStdEncoding, base64.URLEncoding, base64.RawURLEncoding} {
		if b, err := enc.DecodeString(s); err == nil && len(b) == ed25519.PublicKeySize {
			return ed25519.PublicKey(b), nil
		}
	}
	return nil, msg.New("config.publicKeyFormat", nil)
}

// ParseRemote lit un document distant. key peut être nil (pas de signature
// exigée) ; policyAllowed : la clé vient d'un administrateur.
func ParseRemote(body []byte, key ed25519.PublicKey, policyAllowed bool) (*RemoteDoc, error) {
	body = bytes.TrimSpace(body)
	doc := &RemoteDoc{}
	payload := body
	if len(body) > 0 && body[0] != '{' {
		p, err := verifyJWS(string(body), key)
		if err != nil {
			return nil, err
		}
		payload, doc.Signed = p, true
	} else if key != nil {
		return nil, msg.New("config.remoteUnsigned", nil)
	}
	t, err := parseTree(payload)
	if err != nil {
		return nil, err
	}
	if k, ok := findKey(t, "policy"); ok {
		if p, ok := t[k].(Tree); ok && doc.Signed && policyAllowed {
			doc.Policy = p
		}
		delete(t, k)
	}
	doc.Config = t
	return doc, nil
}

type jwsHeader struct {
	Alg string `json:"alg"`
}

func verifyJWS(s string, key ed25519.PublicKey) ([]byte, error) {
	parts := strings.Split(s, ".")
	if len(parts) != 3 {
		return nil, errors.New("configuration distante : ni JSON ni JWS compact")
	}
	if key == nil {
		return nil, msg.New("config.remoteNoPublicKey", nil)
	}
	hb, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return nil, msg.New("config.signatureHeader", nil)
	}
	var h jwsHeader
	if json.Unmarshal(hb, &h) != nil || h.Alg != "EdDSA" {
		return nil, msg.New("config.signatureAlgorithm", nil)
	}
	sig, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return nil, errors.New("signature illisible")
	}
	if !ed25519.Verify(key, []byte(parts[0]+"."+parts[1]), sig) {
		return nil, msg.New("config.signatureInvalid", nil)
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, errors.New("signature : contenu illisible")
	}
	return payload, nil
}

// SignJWS signe un document avec une clé privée Ed25519 (JWS compact).
func SignJWS(payload []byte, key ed25519.PrivateKey) string {
	h := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"EdDSA","typ":"JWT"}`))
	p := base64.RawURLEncoding.EncodeToString(payload)
	sig := ed25519.Sign(key, []byte(h+"."+p))
	return h + "." + p + "." + base64.RawURLEncoding.EncodeToString(sig)
}

// remoteCache est le contenu du fichier cache.
type remoteCache struct {
	URL       string    `json:"url"`
	ETag      string    `json:"etag,omitempty"`
	FetchedAt time.Time `json:"fetchedAt"`
	Body      string    `json:"body"`
}

func readCache(path string) (*remoteCache, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var c remoteCache
	if err := json.Unmarshal(b, &c); err != nil {
		return nil, err
	}
	return &c, nil
}

func writeCache(path string, c *remoteCache) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	b, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// errNotModified : le serveur répond 304, le cache est à jour.
var errNotModified = errors.New("non modifiée")

// fetchRemote télécharge le document ; etag évite de le recharger.
func fetchRemote(ctx context.Context, client *http.Client, url, etag string) (body []byte, newETag string, err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, "", err
	}
	req.Header.Set("Accept", "application/json, application/jose")
	req.Header.Set("User-Agent", "Lattis")
	if etag != "" {
		req.Header.Set("If-None-Match", etag)
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotModified {
		return nil, etag, errNotModified
	}
	if resp.StatusCode/100 != 2 {
		return nil, "", fmt.Errorf("HTTP %s", resp.Status)
	}
	body, err = io.ReadAll(io.LimitReader(resp.Body, maxRemoteSize+1))
	if err != nil {
		return nil, "", err
	}
	if len(body) > maxRemoteSize {
		return nil, "", msg.New("config.remoteTooLarge", nil)
	}
	return body, resp.Header.Get("ETag"), nil
}

// httpsOnly refuse une redirection vers autre chose que https.
func httpsOnly(req *http.Request, via []*http.Request) error {
	if req.URL.Scheme != "https" {
		return msg.New("config.redirectNotHttps", nil)
	}
	if len(via) >= 5 {
		return msg.New("json.tooManyRedirects", nil)
	}
	return nil
}
