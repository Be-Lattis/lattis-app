package api

import (
	"encoding/base64"
	"fmt"
	"net/url"
	"strings"
	"sync"

	"github.com/lalamefine/lattis/internal/msg"
	"github.com/lalamefine/lattis/internal/query"
)

// Tuiles des cartes (graphiques de type « map ») : l'interface les demande
// ici plutôt qu'au réseau directement, pour qu'elles passent par la même
// autorisation par hôte que le module http des scripts (première tuile :
// question à l'utilisateur ; refus : la carte garde un fond neutre).
// L'accord ne dépend pas du scripting : en version web sans scripting,
// l'utilisateur autorise lui-même tile.openstreetmap.org. La démo publique
// n'a aucune connexion sortante (query.Guard.Offline) : la tuile est
// refusée sans question (api.tileDemoOffline), et la carte le dit.

// ErrTileOffline refuse les tuiles quand l'hôte n'a pas de réseau (démo
// publique).
var ErrTileOffline = msg.New("api.tileDemoOffline", nil)

// tileAsk est la question posée avant la première tuile, dans la langue de
// la session.
func tileAsk(h string) Dialog {
	l := msg.Current()
	return Dialog{
		Title:   msg.Format(l, "api.tileAskTitle", nil),
		Message: msg.Format(l, "api.tileAsk", msg.Args{"host": h}),
		Fields:  []Field{{Name: "remember", Label: msg.Format(l, "api.tileRemember", nil), Type: "checkbox"}},
		OK:      msg.Format(l, "api.tileAllow", nil),
		Cancel:  msg.Format(l, "api.tileRefuse", nil),
	}
}

// tilesOffline dit si l'hôte de l'application refuse tout réseau.
func (c *Call) tilesOffline() bool {
	g, ok := c.Host.(query.Guarded)
	return ok && g.QueryGuard() != nil && g.QueryGuard().Offline
}

// tileURL est l'adresse des tuiles (OpenStreetMap) ; les tests la changent.
var tileURL = "https://tile.openstreetmap.org/%d/%d/%d.png"

// Cache des tuiles déjà reçues (z/x/y → data URL), partagé entre classeurs :
// l'autorisation de l'hôte est vérifiée à chaque demande, cache ou non.
var (
	tileMu    sync.Mutex
	tileCache = map[string]string{}
	tileOrder []string
)

const tileCacheMax = 1024

func tileGet(k string) (string, bool) {
	tileMu.Lock()
	defer tileMu.Unlock()
	v, ok := tileCache[k]
	return v, ok
}

func tilePut(k, v string) {
	tileMu.Lock()
	defer tileMu.Unlock()
	if _, ok := tileCache[k]; ok {
		return
	}
	if len(tileOrder) >= tileCacheMax {
		delete(tileCache, tileOrder[0])
		tileOrder = tileOrder[1:]
	}
	tileCache[k] = v
	tileOrder = append(tileOrder, k)
}

// mapTile renvoie la tuile z/x/y en data URL.
func (c *Call) mapTile(z, x, y int) (string, error) {
	if z < 0 || z > 19 || x < 0 || y < 0 || x >= 1<<z || y >= 1<<z {
		return "", msg.New("api.tileOutOfMap", nil)
	}
	raw := fmt.Sprintf(tileURL, z, x, y)
	u, err := url.Parse(raw)
	if err != nil {
		return "", err
	}
	if c.tilesOffline() {
		return "", ErrTileOffline
	}
	if err := c.allowHostAsk(u.Host, tileAsk); err != nil {
		return "", err
	}
	if v, ok := tileGet(raw); ok {
		return v, nil
	}
	r, err := c.doHTTP(httpRequest{method: "GET", url: raw, headers: map[string]string{
		"User-Agent": "Lattis (tableur ; cartes des graphiques)",
	}})
	if err != nil {
		return "", err
	}
	if !r.OK {
		return "", msg.New("api.tileStatus", msg.Args{"z": z, "x": x, "y": y, "status": r.Status})
	}
	ctype := r.Headers["content-type"]
	if !strings.HasPrefix(ctype, "image/") {
		ctype = "image/png"
	}
	v := "data:" + ctype + ";base64," + base64.StdEncoding.EncodeToString([]byte(r.Body))
	tilePut(raw, v)
	return v, nil
}

func init() {
	register(&Command{
		Name:    "charts.tile",
		Summary: "Renvoie une tuile OpenStreetMap (z/x/y) en data URL, pour les graphiques de type carte.",
		Details: "Soumise à l'autorisation de l'hôte tile.openstreetmap.org, comme le module http.",
		Params: []Param{
			{Name: "z", Type: Integer, Desc: "niveau de zoom (0 à 19)"},
			{Name: "x", Type: Integer, Desc: "colonne de la tuile"},
			{Name: "y", Type: Integer, Desc: "ligne de la tuile"},
		},
		NoLua: true,
		NoMCP: true,
		Run: func(c *Call) (any, error) {
			z, err := c.Int("z", 0)
			if err != nil {
				return nil, err
			}
			x, err := c.Int("x", 0)
			if err != nil {
				return nil, err
			}
			y, err := c.Int("y", 0)
			if err != nil {
				return nil, err
			}
			return c.mapTile(z, x, y)
		},
	})
}
