package update

import (
	"context"
	"crypto/ed25519"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"runtime"
	"strings"

	"github.com/lalamefine/lattis/internal/msg"
)

// Repo : dépôt GitHub des versions de l'application.
const Repo = "Be-Lattis/lattis-app"

// GitHubAPI : adresse de l'API GitHub (remplacée dans les tests).
const GitHubAPI = "https://api.github.com"

// Release est une version publiée sur GitHub.
type Release struct {
	Tag        string  `json:"tag_name"`
	Prerelease bool    `json:"prerelease"`
	Draft      bool    `json:"draft"`
	URL        string  `json:"html_url"` // page de la version (notes)
	Assets     []Asset `json:"assets"`
}

// Asset est un fichier d'une version.
type Asset struct {
	Name string `json:"name"`
	URL  string `json:"browser_download_url"`
	Size int64  `json:"size"`
}

// Asset rend le fichier de ce nom (sans tenir compte de la casse).
func (r *Release) Asset(name string) (Asset, bool) {
	for _, a := range r.Assets {
		if strings.EqualFold(a.Name, name) {
			return a, true
		}
	}
	return Asset{}, false
}

// Version : la version du tag ; faux si le tag n'en est pas une.
func (r *Release) Version() (Version, bool) { return ParseVersion(r.Tag) }

// Newest choisit, parmi les versions publiées, la plus récente qui soit
// strictement plus récente que current, ou nil. Une version courante qui est
// une pré-version (canal bêta : 0.3.0-beta) accepte les pré-versions ; une
// version publiée n'accepte que des versions publiées. Brouillons et tags
// qui ne sont pas des versions sont ignorés.
func Newest(releases []Release, current Version) *Release {
	beta := current.Prerelease()
	var best *Release
	var bestV Version
	for i := range releases {
		r := &releases[i]
		v, ok := r.Version()
		if !ok || r.Draft {
			continue
		}
		if (r.Prerelease || v.Prerelease()) && !beta {
			continue
		}
		if !Newer(v, current) {
			continue
		}
		if best == nil || Newer(v, bestV) {
			best, bestV = r, v
		}
	}
	return best
}

// Client fait les requêtes vers GitHub : HTTPS seulement (redirections
// comprises), sans cookie ni identifiant.
type Client struct {
	HTTP    *http.Client
	API     string // GitHubAPI par défaut
	Repo    string // Repo par défaut
	Version string // version de l'application (User-Agent)
	// Keys : clés publiques dont latest.json doit porter la signature
	// (latest.json.sig) ; vide : aucune exigée (voir sign.go).
	Keys []ed25519.PublicKey
}

func (c *Client) api() string {
	if c.API != "" {
		return strings.TrimRight(c.API, "/")
	}
	return GitHubAPI
}

func (c *Client) repo() string {
	if c.Repo != "" {
		return c.Repo
	}
	return Repo
}

// httpClient : le client donné, avec un refus des redirections hors HTTPS.
func (c *Client) httpClient() *http.Client {
	base := c.HTTP
	if base == nil {
		base = &http.Client{}
	}
	hc := *base
	hc.Jar = nil // jamais de cookie
	hc.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if req.URL.Scheme != "https" {
			return msg.New("update.notHttps", msg.Args{"url": req.URL.Redacted()})
		}
		if len(via) >= 10 {
			return msg.New("update.httpStatus", msg.Args{"status": "redirections"})
		}
		return nil
	}
	return &hc
}

// UserAgent : seul renseignement envoyé, la version et le système.
func (c *Client) UserAgent() string {
	v := c.Version
	if v == "" {
		v = "dev"
	}
	return "Lattis/" + v + " (" + runtime.GOOS + ")"
}

// get fait un GET https et rend la réponse (statut 200) ; à fermer.
func (c *Client) get(ctx context.Context, rawURL, accept string) (*http.Response, error) {
	u, err := url.Parse(rawURL)
	if err != nil || u.Scheme != "https" || u.Host == "" {
		return nil, msg.New("update.notHttps", msg.Args{"url": rawURL})
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", c.UserAgent())
	if accept != "" {
		req.Header.Set("Accept", accept)
	}
	resp, err := c.httpClient().Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		switch {
		case resp.StatusCode == http.StatusTooManyRequests,
			resp.StatusCode == http.StatusForbidden && resp.Header.Get("X-RateLimit-Remaining") == "0":
			return nil, msg.New("update.rateLimited", nil)
		case resp.StatusCode == http.StatusProxyAuthRequired:
			return nil, msg.New("update.proxyAuth", nil)
		}
		return nil, msg.New("update.httpStatus", msg.Args{"status": resp.Status})
	}
	return resp, nil
}

// getSmall lit une réponse entière de max octets au plus.
func (c *Client) getSmall(ctx context.Context, rawURL, accept string, max int64) ([]byte, error) {
	resp, err := c.get(ctx, rawURL, accept)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(io.LimitReader(resp.Body, max+1))
	if err != nil {
		return nil, err
	}
	if int64(len(b)) > max {
		return nil, msg.New("update.tooLarge", nil)
	}
	return b, nil
}

// maxListing : taille maximale d'une réponse de l'API (liste des versions).
const maxListing = 4 << 20

// Releases liste les dernières versions publiées (API GitHub, 60 appels par
// heure sans jeton : la recherche se fait au plus deux fois par jour).
func (c *Client) Releases(ctx context.Context) ([]Release, error) {
	b, err := c.getSmall(ctx, fmt.Sprintf("%s/repos/%s/releases?per_page=20", c.api(), c.repo()), "application/vnd.github+json", maxListing)
	if err != nil {
		return nil, err
	}
	var out []Release
	if err := json.Unmarshal(b, &out); err != nil {
		return nil, msg.New("update.badResponse", msg.Args{"err": err.Error()})
	}
	return out, nil
}

// ReleaseByTag rend une version par son tag.
func (c *Client) ReleaseByTag(ctx context.Context, tag string) (*Release, error) {
	b, err := c.getSmall(ctx, fmt.Sprintf("%s/repos/%s/releases/tags/%s", c.api(), c.repo(), url.PathEscape(tag)), "application/vnd.github+json", maxListing)
	if err != nil {
		return nil, err
	}
	var r Release
	if err := json.Unmarshal(b, &r); err != nil {
		return nil, msg.New("update.badResponse", msg.Args{"err": err.Error()})
	}
	return &r, nil
}

// ReleasePage : adresse de la page d'une version sur GitHub (notes).
func ReleasePage(repo, version string) string {
	if repo == "" {
		repo = Repo
	}
	tag := version
	if !strings.HasPrefix(tag, "v") {
		tag = "v" + tag
	}
	return "https://github.com/" + repo + "/releases/tag/" + tag
}
