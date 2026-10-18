package query

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/lalamefine/lattis/internal/grid"
)

// Sources JSON : fichier (.json, ou une valeur par ligne : NDJSON) et API
// web (GET d'une adresse). Le chemin des enregistrements (data.items,
// $.resultats[0].lignes) désigne le tableau d'objets à lire ; vide : la
// racine, ou son seul tableau. Les objets imbriqués sont aplatis en
// colonnes « parent.enfant » ; les tableaux imbriqués restent du texte
// JSON. Les colonnes suivent l'ordre de première apparition des clés.

// Header est un en-tête HTTP d'une source web (enregistré dans le
// classeur : un secret passe par une connexion Web).
type Header struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// Limites des sources JSON.
const (
	maxJSONBytes = 512 << 20
	webTimeout   = 2 * time.Minute
)

// jsonObject est un objet JSON qui garde l'ordre de ses clés.
type jsonObject struct {
	keys []string
	vals map[string]any
}

// parseJSON lit une ou plusieurs valeurs JSON (plusieurs : NDJSON, rendues
// comme un tableau). Les nombres restent des json.Number.
func parseJSON(r io.Reader) (any, error) {
	dec := json.NewDecoder(bufio.NewReader(io.LimitReader(r, maxJSONBytes+1)))
	dec.UseNumber()
	var all []any
	for {
		v, err := readJSON(dec)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			if len(all) == 0 {
				return nil, fmt.Errorf("JSON illisible : %w", err)
			}
			return nil, fmt.Errorf("JSON illisible (valeur %d) : %w", len(all)+1, err)
		}
		all = append(all, v)
	}
	switch len(all) {
	case 0:
		return nil, errors.New("JSON vide")
	case 1:
		return all[0], nil
	}
	return all, nil
}

func readJSON(dec *json.Decoder) (any, error) {
	t, err := dec.Token()
	if err != nil {
		return nil, err
	}
	switch x := t.(type) {
	case json.Delim:
		switch x {
		case '{':
			o := &jsonObject{vals: map[string]any{}}
			for dec.More() {
				kt, err := dec.Token()
				if err != nil {
					return nil, unexpectedEOF(err)
				}
				k, _ := kt.(string)
				v, err := readJSON(dec)
				if err != nil {
					return nil, unexpectedEOF(err)
				}
				if _, dup := o.vals[k]; !dup {
					o.keys = append(o.keys, k)
				}
				o.vals[k] = v
			}
			if _, err := dec.Token(); err != nil {
				return nil, unexpectedEOF(err)
			}
			return o, nil
		case '[':
			a := []any{}
			for dec.More() {
				v, err := readJSON(dec)
				if err != nil {
					return nil, unexpectedEOF(err)
				}
				a = append(a, v)
			}
			if _, err := dec.Token(); err != nil {
				return nil, unexpectedEOF(err)
			}
			return a, nil
		}
		return nil, fmt.Errorf("délimiteur inattendu %q", x)
	}
	return t, nil
}

func unexpectedEOF(err error) error {
	if errors.Is(err, io.EOF) {
		return io.ErrUnexpectedEOF
	}
	return err
}

// pathPart découpe un chemin d'enregistrements : clés séparées par des
// points, indices entre crochets, clés entre crochets et guillemets.
var pathPart = regexp.MustCompile(`\[\s*(\d+)\s*\]|\[\s*'([^']*)'\s*\]|\[\s*"([^"]*)"\s*\]|([^.\[\]]+)`)

// selectRecords suit le chemin path et rend les enregistrements.
func selectRecords(root any, path string) ([]any, error) {
	p := strings.TrimSpace(path)
	p = strings.TrimPrefix(p, "$")
	cur := root
	where := "$"
	for _, m := range pathPart.FindAllStringSubmatch(p, -1) {
		switch {
		case m[1] != "":
			i, _ := strconv.Atoi(m[1])
			a, ok := cur.([]any)
			if !ok {
				return nil, fmt.Errorf("chemin %s : %s n'est pas un tableau", path, where)
			}
			if i >= len(a) {
				return nil, fmt.Errorf("chemin %s : %s n'a que %d élément(s)", path, where, len(a))
			}
			cur = a[i]
			where += "[" + m[1] + "]"
		default:
			k := m[2] + m[3] + m[4]
			if m[4] != "" {
				k = strings.TrimSpace(k)
			}
			o, ok := cur.(*jsonObject)
			if !ok {
				return nil, fmt.Errorf("chemin %s : %s n'est pas un objet", path, where)
			}
			v, ok := o.vals[k]
			if !ok {
				return nil, fmt.Errorf("chemin %s : clé « %s » introuvable dans %s (clés : %s)", path, k, where, keyList(o))
			}
			cur = v
			where += "." + k
		}
	}
	switch x := cur.(type) {
	case []any:
		return x, nil
	case *jsonObject:
		if p == "" {
			// Racine objet : son seul tableau ({"data": [...]}), sinon
			// l'objet lui-même comme unique enregistrement.
			var only []any
			n := 0
			for _, k := range x.keys {
				if a, ok := x.vals[k].([]any); ok {
					only = a
					n++
				}
			}
			if n == 1 {
				return only, nil
			}
		}
		return []any{x}, nil
	case nil:
		return []any{}, nil
	}
	return nil, fmt.Errorf("chemin %s : %s n'est ni un tableau ni un objet", path, where)
}

func keyList(o *jsonObject) string {
	ks := o.keys
	more := ""
	if len(ks) > 12 {
		ks, more = ks[:12], "…"
	}
	return strings.Join(ks, ", ") + more
}

// flatRecords aplatit des enregistrements : noms des colonnes (ordre de
// première apparition) et valeurs par ligne.
type flatRecords struct {
	cols []string
	idx  map[string]int
	rows [][]any
}

func (f *flatRecords) col(name string) int {
	if i, ok := f.idx[name]; ok {
		return i
	}
	f.idx[name] = len(f.cols)
	f.cols = append(f.cols, name)
	return len(f.cols) - 1
}

func flatten(records []any) *flatRecords {
	f := &flatRecords{idx: map[string]int{}}
	for _, rec := range records {
		row := map[int]any{}
		var walk func(prefix string, v any)
		walk = func(prefix string, v any) {
			switch x := v.(type) {
			case *jsonObject:
				if len(x.keys) == 0 && prefix != "" {
					row[f.col(strings.TrimSuffix(prefix, "."))] = nil
					return
				}
				for _, k := range x.keys {
					name := prefix + k
					if name == "" {
						name = "Colonne"
					}
					if o, ok := x.vals[k].(*jsonObject); ok {
						walk(name+".", o)
						continue
					}
					row[f.col(name)] = scalarJSON(x.vals[k])
				}
			case []any:
				if prefix == "" {
					// Tableau de tableaux : Colonne 1, Colonne 2…
					for i, e := range x {
						row[f.col(fmt.Sprintf("Colonne %d", i+1))] = scalarJSON(e)
					}
					return
				}
				row[f.col(strings.TrimSuffix(prefix, "."))] = scalarJSON(x)
			default:
				name := strings.TrimSuffix(prefix, ".")
				if name == "" {
					name = "Valeur"
				}
				row[f.col(name)] = scalarJSON(x)
			}
		}
		walk("", rec)
		out := make([]any, len(f.cols))
		for i, v := range row {
			out[i] = v
		}
		f.rows = append(f.rows, out)
	}
	return f
}

// scalarJSON garde une valeur simple (nil, bool, json.Number, string) ;
// un tableau ou un objet devient son texte JSON.
func scalarJSON(v any) any {
	switch v.(type) {
	case []any, *jsonObject:
		var b bytes.Buffer
		writeJSON(&b, v)
		return b.String()
	}
	return v
}

func writeJSON(b *bytes.Buffer, v any) {
	switch x := v.(type) {
	case nil:
		b.WriteString("null")
	case bool:
		b.WriteString(strconv.FormatBool(x))
	case json.Number:
		b.WriteString(x.String())
	case string:
		s, _ := json.Marshal(x)
		b.Write(s)
	case []any:
		b.WriteByte('[')
		for i, e := range x {
			if i > 0 {
				b.WriteByte(',')
			}
			writeJSON(b, e)
		}
		b.WriteByte(']')
	case *jsonObject:
		b.WriteByte('{')
		for i, k := range x.keys {
			if i > 0 {
				b.WriteByte(',')
			}
			s, _ := json.Marshal(k)
			b.Write(s)
			b.WriteByte(':')
			writeJSON(b, x.vals[k])
		}
		b.WriteByte('}')
	}
}

var isoDate = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)

// jsonType donne le type d'une colonne d'après toutes ses valeurs : entier,
// décimal, booléen, date (AAAA-MM-JJ) si toutes s'y prêtent, sinon texte.
func jsonType(rows [][]any, col int) grid.Type {
	ints, nums, bools, dates, n := true, true, true, true, 0
	for _, r := range rows {
		if col >= len(r) || r[col] == nil {
			continue
		}
		n++
		switch x := r[col].(type) {
		case json.Number:
			bools, dates = false, false
			if _, err := strconv.ParseInt(x.String(), 10, 64); err != nil {
				ints = false
			}
		case bool:
			ints, nums, dates = false, false, false
		case string:
			ints, nums, bools = false, false, false
			if !isoDate.MatchString(x) {
				dates = false
			} else if _, err := time.Parse("2006-01-02", x); err != nil {
				dates = false
			}
		default:
			return grid.Text
		}
	}
	switch {
	case n == 0:
		return grid.Text
	case ints:
		return grid.Integer
	case nums:
		return grid.Decimal
	case bools:
		return grid.Boolean
	case dates:
		return grid.Date
	}
	return grid.Text
}

// jsonCell ramène une valeur JSON au stockage du type de sa colonne.
func jsonCell(v any, t grid.Type) any {
	switch x := v.(type) {
	case nil:
		return nil
	case json.Number:
		switch t {
		case grid.Integer:
			if n, err := x.Int64(); err == nil {
				return n
			}
		case grid.Decimal:
			if f, err := x.Float64(); err == nil {
				return f
			}
		}
		return x.String()
	case bool:
		if t == grid.Boolean {
			if x {
				return int64(1)
			}
			return int64(0)
		}
		return strconv.FormatBool(x)
	}
	return v
}

// storeRecords charge des enregistrements JSON dans la base de travail.
func (r *run) storeRecords(records []any) (loaded, error) {
	if r.limit > 0 && len(records) > r.limit {
		records = records[:r.limit+1] // une de plus : l'aperçu se sait tronqué
	}
	f := flatten(records)
	if len(f.cols) == 0 {
		return loaded{}, errors.New("aucun enregistrement (vérifiez le chemin des enregistrements)")
	}
	cols := make([]Column, len(f.cols))
	for i, name := range f.cols {
		cols[i] = Column{Name: name, Type: jsonType(f.rows, i)}
	}
	s, err := r.begin(cols)
	if err != nil {
		return loaded{}, err
	}
	row := make([]any, len(cols))
	for _, fr := range f.rows {
		for i := range row {
			row[i] = nil
			if i < len(fr) {
				row[i] = jsonCell(fr[i], cols[i].Type)
			}
		}
		if err = s.insert(row); err != nil {
			break
		}
	}
	return s.finish(err)
}

func (r *run) loadJSON(src Source) (loaded, error) {
	if src.Path == "" {
		return loaded{}, errors.New("fichier JSON non choisi")
	}
	f, err := os.Open(src.Path)
	if err != nil {
		return loaded{}, err
	}
	defer f.Close()
	root, err := parseJSON(f)
	if err != nil {
		return loaded{}, fmt.Errorf("%s : %w", baseName(src.Path), err)
	}
	recs, err := selectRecords(root, src.RecordPath)
	if err != nil {
		return loaded{}, err
	}
	return r.storeRecords(recs)
}

func (r *run) loadWeb(src Source) (loaded, error) {
	var conn *Connection
	if strings.TrimSpace(src.Connection) != "" {
		c, err := r.connection(src.Connection)
		if err != nil {
			return loaded{}, err
		}
		conn = &c
	}
	body, err := fetchWeb(r.ctx, r.env.secrets(), conn, src.URL, src.Headers, "")
	if err != nil {
		return loaded{}, err
	}
	root, err := parseJSON(bytes.NewReader(body))
	if err != nil {
		return loaded{}, fmt.Errorf("réponse de %s : %w", hostOf(src.URL), err)
	}
	recs, err := selectRecords(root, src.RecordPath)
	if err != nil {
		return loaded{}, err
	}
	return r.storeRecords(recs)
}

func hostOf(u string) string {
	if p, err := url.Parse(u); err == nil && p.Host != "" {
		return p.Host
	}
	return u
}

// webURL vérifie une adresse http(s).
func webURL(raw string) (*url.URL, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return nil, errors.New("adresse web invalide : elle doit commencer par http:// ou https://")
	}
	return u, nil
}

// underPrefix : l'adresse u est couverte par le préfixe de la connexion
// (même schéma, même hôte, chemin qui commence par le sien) : le secret
// n'est jamais envoyé ailleurs.
func underPrefix(u *url.URL, prefix string) bool {
	p, err := url.Parse(strings.TrimSpace(prefix))
	if err != nil {
		return false
	}
	if !strings.EqualFold(u.Scheme, p.Scheme) || !strings.EqualFold(u.Host, p.Host) {
		return false
	}
	pp := strings.TrimSuffix(p.Path, "/")
	return pp == "" || u.Path == pp || strings.HasPrefix(u.Path, pp+"/")
}

// webAuth pose le secret d'une connexion Web sur une requête :
// authentification Basic si la connexion a un utilisateur, sinon l'en-tête
// de la connexion (Authorization par défaut, « Bearer » ajouté à un jeton
// nu).
func webAuth(req *http.Request, c DBConfig, secret string) string {
	if c.User != "" {
		req.SetBasicAuth(c.User, secret)
		return "Authorization"
	}
	h := strings.TrimSpace(c.Header)
	if h == "" {
		h = "Authorization"
	}
	if strings.EqualFold(h, "Authorization") && !strings.Contains(strings.TrimSpace(secret), " ") {
		secret = "Bearer " + strings.TrimSpace(secret)
	}
	req.Header.Set(h, secret)
	return h
}

// httpError : le serveur a répondu, mais pas 2xx.
type httpError struct {
	Host, Status, Message string
}

func (e *httpError) Error() string { return fmt.Sprintf("%s : HTTP %s%s", e.Host, e.Status, e.Message) }

// fetchWeb lit une adresse (GET). conn (facultative) fournit le secret ;
// password non vide remplace celui qui est connu (test de la connexion).
func fetchWeb(ctx context.Context, sec *Secrets, conn *Connection, raw string, headers []Header, password string) ([]byte, error) {
	u, err := webURL(raw)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "Lattis")
	for _, h := range headers {
		if n := strings.TrimSpace(h.Name); n != "" {
			req.Header.Set(n, h.Value)
		}
	}
	authHeader, secret := "", password
	if conn != nil {
		if !underPrefix(u, conn.Config.URL) {
			return nil, fmt.Errorf("l'adresse %s n'est pas couverte par la connexion « %s » (%s) : son secret n'y est pas envoyé", u.Redacted(), conn.Name, conn.Config.URL)
		}
		if secret == "" {
			secret, _ = sec.Get(conn.Account())
			if secret == "" && sec.Pending(conn.Account()) {
				return nil, &ErrPassword{Connection: conn.Name, Account: conn.Account(), Message: "secret enregistré à autoriser", Stored: true}
			}
		}
		if secret != "" {
			authHeader = webAuth(req, conn.Config, secret)
		}
	}
	client := &http.Client{
		Timeout: webTimeout,
		CheckRedirect: func(next *http.Request, via []*http.Request) error {
			if len(via) >= 10 {
				return errors.New("trop de redirections")
			}
			// Le secret ne suit pas une redirection vers un autre hôte.
			if authHeader != "" && !strings.EqualFold(next.URL.Host, via[0].URL.Host) {
				next.Header.Del(authHeader)
			}
			return nil
		},
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%s : %w", u.Host, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		if conn != nil {
			msg := "aucun secret enregistré"
			if secret != "" {
				msg = "secret refusé (HTTP " + strconv.Itoa(resp.StatusCode) + ")"
			}
			return nil, &ErrPassword{Connection: conn.Name, Account: conn.Account(), Message: msg}
		}
		return nil, fmt.Errorf("%s : HTTP %d, authentification requise (choisissez une connexion Web qui porte le jeton)", u.Host, resp.StatusCode)
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		snippet, _ := io.ReadAll(io.LimitReader(resp.Body, 300))
		msg := strings.Join(strings.Fields(string(snippet)), " ")
		if msg != "" {
			msg = " : " + msg
		}
		return nil, &httpError{Host: u.Host, Status: resp.Status, Message: msg}
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxJSONBytes+1))
	if err != nil {
		return nil, fmt.Errorf("%s : %w", u.Host, err)
	}
	if len(body) > maxJSONBytes {
		return nil, fmt.Errorf("%s : réponse de plus de %d Mo", u.Host, maxJSONBytes>>20)
	}
	return body, nil
}

// testWeb essaie une connexion Web : GET de son adresse, avec password
// (vide : le secret connu). Seul un refus (401, 403) ou une panne réseau
// est une erreur : l'adresse de base d'une API peut répondre 404.
func testWeb(ctx context.Context, sec *Secrets, c Connection, password string, remember bool) error {
	_, err := fetchWeb(ctx, sec, &c, c.Config.URL, nil, password)
	var he *httpError
	if err != nil && !errors.As(err, &he) {
		return err
	}
	if password != "" {
		return sec.Set(c.Account(), password, remember)
	}
	return nil
}
