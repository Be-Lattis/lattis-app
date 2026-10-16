package canvas

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

// Favorite est une requête favorite de l'onglet SQL, enregistrée dans le
// classeur : son nom et son texte tel que saisi (noms affichés).
type Favorite struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
	SQL  string `json:"sql"`
}

// Favorites liste les requêtes favorites, dans l'ordre d'ajout.
func (m *Model) Favorites(ctx context.Context) ([]Favorite, error) {
	out := []Favorite{}
	err := m.read(ctx, func(t *tx) error {
		fs, err := t.favorites()
		out = append(out, fs...)
		return err
	})
	return out, err
}

// favoriteName propose un nom à une requête : sa première ligne utile.
func favoriteName(q string) string {
	for _, l := range strings.Split(q, "\n") {
		l = strings.TrimSpace(l)
		if l == "" || strings.HasPrefix(l, "--") {
			continue
		}
		if r := []rune(l); len(r) > 60 {
			l = string(r[:60]) + "…"
		}
		return l
	}
	return "Requête"
}

// sameSQL compare deux requêtes sans tenir compte des espaces.
func sameSQL(a, b string) bool {
	return strings.Join(strings.Fields(a), " ") == strings.Join(strings.Fields(b), " ")
}

// AddFavorite ajoute une requête aux favoris (nom vide : sa première ligne)
// et renvoie son identifiant ; une requête déjà favorite garde le sien.
func (m *Model) AddFavorite(ctx context.Context, name, q string) (int64, error) {
	if strings.TrimSpace(q) == "" {
		return 0, errors.New("favori : requête vide")
	}
	name = strings.TrimSpace(name)
	if name == "" {
		name = favoriteName(q)
	}
	var id int64
	err := m.Do(ctx, "Ajouter aux favoris", func(t *tx) error {
		have, err := t.favorites()
		if err != nil {
			return err
		}
		for _, f := range have {
			if sameSQL(f.SQL, q) {
				id = f.ID
				return nil
			}
		}
		if err := t.row(`SELECT coalesce(max(id), 0) + 1 FROM _slitable_favorites`).Scan(&id); err != nil {
			return err
		}
		return t.apply(
			[]stmt{st(`INSERT INTO _slitable_favorites (id, name, sql) VALUES (?, ?, ?)`, id, name, q)},
			[]stmt{st(`DELETE FROM _slitable_favorites WHERE id = ?`, id)},
		)
	})
	return id, err
}

func (t *tx) favorites() ([]Favorite, error) {
	rs, err := t.query(`SELECT id, name, sql FROM _slitable_favorites ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rs.Close()
	var out []Favorite
	for rs.Next() {
		var f Favorite
		if err := rs.Scan(&f.ID, &f.Name, &f.SQL); err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, rs.Err()
}

// favorite lit un favori.
func (t *tx) favorite(id int64) (Favorite, error) {
	f := Favorite{ID: id}
	err := t.row(`SELECT name, sql FROM _slitable_favorites WHERE id = ?`, id).Scan(&f.Name, &f.SQL)
	if errors.Is(err, sql.ErrNoRows) {
		return f, fmt.Errorf("favori %d introuvable", id)
	}
	return f, err
}

// RenameFavorite renomme une requête favorite.
func (m *Model) RenameFavorite(ctx context.Context, id int64, name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return errors.New("favori : nom vide")
	}
	return m.Do(ctx, "Renommer le favori", func(t *tx) error {
		f, err := t.favorite(id)
		if err != nil || f.Name == name {
			return err
		}
		q := `UPDATE _slitable_favorites SET name = ? WHERE id = ?`
		return t.apply([]stmt{st(q, name, id)}, []stmt{st(q, f.Name, id)})
	})
}

// DeleteFavorite retire une requête des favoris.
func (m *Model) DeleteFavorite(ctx context.Context, id int64) error {
	return m.Do(ctx, "Retirer des favoris", func(t *tx) error {
		f, err := t.favorite(id)
		if err != nil {
			return err
		}
		return t.apply(
			[]stmt{st(`DELETE FROM _slitable_favorites WHERE id = ?`, id)},
			[]stmt{st(`INSERT INTO _slitable_favorites (id, name, sql) VALUES (?, ?, ?)`, id, f.Name, f.SQL)},
		)
	})
}
