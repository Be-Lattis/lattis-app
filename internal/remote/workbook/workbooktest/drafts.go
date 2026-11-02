package workbooktest

import (
	"bytes"
	"context"
	"slices"
	"sync"
	"time"

	"github.com/lalamefine/lattis/internal/remote/workbook"
)

// ErrSQL imite une erreur de PostgreSQL avec son code (comme pgconn.PgError).
type ErrSQL struct{ Code string }

func (e ErrSQL) Error() string { return "ERROR (SQLSTATE " + e.Code + ")" }

// SQLState rend le code de l'erreur.
func (e ErrSQL) SQLState() string { return e.Code }

// Drafts garde des brouillons en mémoire (workbook.DraftStore).
type Drafts struct {
	mu   sync.Mutex
	rows map[string]*workbook.DraftInfo
	data map[string][]byte
	// Quota : limite en octets, tous brouillons confondus (0 : aucune).
	Quota int64
	// Fail : erreur rendue par tous les appels (serveur injoignable…).
	Fail error
	// Gate, s'il est posé : Save attend d'en recevoir une valeur avant
	// d'écrire (envoi lent).
	Gate chan struct{}
	// Saves : contenu des envois réussis, dans l'ordre ; Tries : tous les essais.
	Saves [][]byte
	Tries int
	// Started est prévenu au début de chaque Save, s'il est posé.
	Started chan struct{}
}

// NewDrafts donne une réserve vide.
func NewDrafts() *Drafts {
	return &Drafts{rows: map[string]*workbook.DraftInfo{}, data: map[string][]byte{}}
}

// SetFail change l'erreur rendue par tous les appels.
func (f *Drafts) SetFail(err error) {
	f.mu.Lock()
	f.Fail = err
	f.mu.Unlock()
}

// Counts rend le nombre d'essais et d'envois réussis.
func (f *Drafts) Counts() (tries, saves int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.Tries, len(f.Saves)
}

// Last rend le dernier envoi réussi.
func (f *Drafts) Last() []byte {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.Saves) == 0 {
		return nil
	}
	return f.Saves[len(f.Saves)-1]
}

// IDs rend les identifiants des brouillons gardés.
func (f *Drafts) IDs() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []string
	for id := range f.rows {
		out = append(out, id)
	}
	slices.Sort(out)
	return out
}

func (f *Drafts) Save(ctx context.Context, id, title string, data []byte) (time.Time, error) {
	f.mu.Lock()
	f.Tries++
	started, gate, fail := f.Started, f.Gate, f.Fail
	f.mu.Unlock()
	if started != nil {
		select {
		case started <- struct{}{}:
		default:
		}
	}
	if gate != nil {
		select {
		case <-gate:
		case <-ctx.Done():
			return time.Time{}, ctx.Err()
		}
	}
	if fail != nil {
		return time.Time{}, fail
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.Quota > 0 {
		used := int64(len(data))
		for k, b := range f.data {
			if k != id {
				used += int64(len(b))
			}
		}
		if used > f.Quota {
			return time.Time{}, ErrSQL{Code: "LT001"}
		}
	}
	at := time.Now()
	f.rows[id] = &workbook.DraftInfo{ID: id, Title: title, Size: int64(len(data)), UpdatedAt: at}
	f.data[id] = bytes.Clone(data)
	f.Saves = append(f.Saves, bytes.Clone(data))
	return at, nil
}

func (f *Drafts) List(context.Context) ([]workbook.DraftInfo, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.Fail != nil {
		return nil, f.Fail
	}
	out := []workbook.DraftInfo{}
	for _, r := range f.rows {
		out = append(out, *r)
	}
	slices.SortFunc(out, func(a, b workbook.DraftInfo) int { return b.UpdatedAt.Compare(a.UpdatedAt) })
	return out, nil
}

func (f *Drafts) Read(_ context.Context, id string) ([]byte, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.Fail != nil {
		return nil, f.Fail
	}
	b, ok := f.data[id]
	if !ok {
		return nil, workbook.ErrDraftGone
	}
	return bytes.Clone(b), nil
}

func (f *Drafts) Delete(_ context.Context, id string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.Fail != nil {
		return f.Fail
	}
	delete(f.rows, id)
	delete(f.data, id)
	return nil
}
