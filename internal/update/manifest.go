package update

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/lalamefine/lattis/internal/msg"
)

// Manifeste d'une version (latest.json, scripts/release-manifest.sh,
// format décrit dans docs/release/publier.md).
type Manifest struct {
	Version    string                  `json:"version"`
	Tag        string                  `json:"tag"`
	Prerelease bool                    `json:"prerelease"`
	ReleaseURL string                  `json:"release_url"`
	Files      map[string]ManifestFile `json:"files"`
}

// ManifestFile décrit un fichier de la version.
type ManifestFile struct {
	Name   string `json:"name"`
	URL    string `json:"url"`
	SHA256 string `json:"sha256"`
	Size   int64  `json:"size"`
}

// Expected est ce qu'un fichier téléchargé doit être.
type Expected struct {
	Name   string
	SHA256 string // 64 caractères hexadécimaux, minuscules
	Size   int64  // 0 : inconnue
}

// Noms des fichiers de contrôle publiés avec chaque version.
const (
	ManifestName = "latest.json"
	SumsName     = "SHA256SUMS.txt"
)

// maxManifest : taille maximale de latest.json et de SHA256SUMS.txt.
const maxManifest = 64 << 10

// MaxDownload : taille maximale d'un fichier de mise à jour.
const MaxDownload = 512 << 20

// ParseManifest lit latest.json et vérifie qu'il décrit bien la version tag.
func ParseManifest(b []byte, tag string) (*Manifest, error) {
	var m Manifest
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, msg.New("update.badManifest", msg.Args{"err": err.Error()})
	}
	want, ok := ParseVersion(tag)
	got, ok2 := ParseVersion(m.Version)
	if !ok || !ok2 || Compare(want, got) != 0 {
		return nil, msg.New("update.manifestVersion", msg.Args{"version": m.Version, "tag": tag})
	}
	return &m, nil
}

// File rend l'entrée du manifeste pour un fichier, par son nom.
func (m *Manifest) File(name string) (Expected, bool) {
	for _, f := range m.Files {
		if strings.EqualFold(f.Name, name) && validSHA(f.SHA256) {
			return Expected{Name: f.Name, SHA256: strings.ToLower(f.SHA256), Size: f.Size}, true
		}
	}
	return Expected{}, false
}

// ParseSums lit SHA256SUMS.txt (sortie de sha256sum : « empreinte  nom »,
// ou « empreinte *nom » en binaire) et rend l'empreinte de name.
func ParseSums(b []byte, name string) (Expected, bool) {
	sc := bufio.NewScanner(bytes.NewReader(b))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		sum, file, ok := strings.Cut(line, " ")
		if !ok {
			continue
		}
		file = strings.TrimPrefix(strings.TrimSpace(file), "*")
		if strings.EqualFold(file, name) && validSHA(sum) {
			return Expected{Name: file, SHA256: strings.ToLower(sum)}, true
		}
	}
	return Expected{}, false
}

func validSHA(s string) bool {
	if len(s) != 64 {
		return false
	}
	_, err := hex.DecodeString(s)
	return err == nil
}

// Expected cherche l'empreinte attendue d'un fichier de la version r :
// d'abord dans latest.json, sinon dans SHA256SUMS.txt. Sans l'un ni
// l'autre, ou si le fichier n'y figure pas, rien ne sera installé.
func (c *Client) Expected(ctx context.Context, r *Release, name string) (Expected, error) {
	if a, ok := r.Asset(ManifestName); ok {
		b, err := c.getSmall(ctx, a.URL, "", maxManifest)
		if err != nil {
			return Expected{}, err
		}
		m, err := ParseManifest(b, r.Tag)
		if err != nil {
			return Expected{}, err
		}
		if e, ok := m.File(name); ok {
			return e, nil
		}
	}
	if a, ok := r.Asset(SumsName); ok {
		b, err := c.getSmall(ctx, a.URL, "", maxManifest)
		if err != nil {
			return Expected{}, err
		}
		if e, ok := ParseSums(b, name); ok {
			return e, nil
		}
	}
	return Expected{}, msg.New("update.noChecksum", msg.Args{"name": name})
}

// Download télécharge url dans dest (remplacé) en calculant l'empreinte au
// fil de l'eau ; le fichier n'est gardé que si sa taille et son empreinte
// sont celles attendues. Le téléchargement passe par un fichier temporaire
// du même dossier, renommé à la fin.
func (c *Client) Download(ctx context.Context, rawURL, dest string, want Expected) error {
	if !validSHA(want.SHA256) {
		return msg.New("update.noChecksum", msg.Args{"name": want.Name})
	}
	if want.Size > MaxDownload {
		return msg.New("update.tooLarge", nil)
	}
	resp, err := c.get(ctx, rawURL, "application/octet-stream")
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.ContentLength > MaxDownload || want.Size > 0 && resp.ContentLength > 0 && resp.ContentLength != want.Size {
		return msg.New("update.sizeMismatch", msg.Args{"name": want.Name})
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(dest), ".lattis-update-*")
	if err != nil {
		return err
	}
	tmp := f.Name()
	keep := false
	defer func() {
		if !keep {
			f.Close()
			os.Remove(tmp)
		}
	}()
	limit := int64(MaxDownload)
	if want.Size > 0 {
		limit = want.Size
	}
	h := sha256.New()
	n, err := io.Copy(io.MultiWriter(f, h), io.LimitReader(resp.Body, limit+1))
	if err != nil {
		return err
	}
	if n > limit || want.Size > 0 && n != want.Size {
		return msg.New("update.sizeMismatch", msg.Args{"name": want.Name})
	}
	if got := hex.EncodeToString(h.Sum(nil)); got != want.SHA256 {
		return msg.New("update.checksumMismatch", msg.Args{"name": want.Name})
	}
	if err := f.Close(); err != nil {
		return err
	}
	os.Remove(dest)
	if err := os.Rename(tmp, dest); err != nil {
		return err
	}
	keep = true
	return nil
}

// FileSHA256 : empreinte d'un fichier (contrôle avant de l'installer).
func FileSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
