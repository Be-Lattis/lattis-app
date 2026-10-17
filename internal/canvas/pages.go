package canvas

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"slices"
)

// Onglets « page HTML » : une feuille dont le contenu est une page HTML
// (avec son CSS et son JavaScript) au lieu d'une grille. L'interface
// l'affiche dans un cadre isolé (sans accès à l'application) ; la page ne
// parle au classeur que par les commandes de l'API (internal/api), comme un
// script. La page est une ligne de _lattis_pages : renommer, déplacer,
// masquer, protéger, copier ou supprimer l'onglet marchent comme pour une
// feuille (la ligne part et revient avec elle, voir sheetPositioned).

// KindHTML est le type d'un onglet page HTML (SheetInfo.Kind).
const KindHTML = "html"

// PageTemplate est le contenu d'une nouvelle page HTML.
const PageTemplate = `<!doctype html>
<html lang="fr">
<head>
<meta charset="utf-8">
<style>
  body { font: 14px system-ui, sans-serif; margin: 24px; color: #222; }
  h1 { font-size: 20px; margin: 0 0 12px; }
  .muted { color: #777; }
</style>
</head>
<body>
<h1>Nouvelle page</h1>
<p class="muted">Modifiez cette page (bouton Modifier) : HTML, CSS et JavaScript.</p>
<p>Feuilles du classeur : <span id="sheets">…</span></p>
<script>
  // lattis.call(commande, arguments) appelle les mêmes commandes que les
  // scripts Lua (sheets.list, db.query, cells.set…) ; lattis.on("change", f)
  // prévient quand le classeur change.
  async function refresh() {
    const sheets = await lattis.call("sheets.list");
    document.getElementById("sheets").textContent = sheets.map((s) => s.name).join(", ");
  }
  lattis.on("change", refresh);
  refresh();
</script>
</body>
</html>
`

// AddPage ajoute un onglet page HTML, juste avant l'onglet before (0 : à
// la fin), et renvoie son identifiant.
func (m *Model) AddPage(ctx context.Context, before int64) (int64, error) {
	var id int64
	err := m.Do(ctx, "Nouvel onglet HTML", func(t *tx) (err error) {
		if id, err = t.addNamedSheet("Page"); err != nil {
			return err
		}
		if err := t.apply(
			[]stmt{st(`INSERT INTO _lattis_pages (sheet, source) VALUES (?, ?)`, id, PageTemplate)},
			[]stmt{st(`DELETE FROM _lattis_pages WHERE sheet = ?`, id)},
		); err != nil {
			return err
		}
		if before == 0 {
			return nil
		}
		ids, err := t.sheetOrder()
		if err != nil {
			return err
		}
		at := slices.Index(ids, before)
		if at < 0 {
			return fmt.Errorf("%w : feuille introuvable", ErrSheet)
		}
		return t.moveSheet(id, at)
	})
	return id, err
}

// IsPage dit si la feuille est un onglet page HTML.
func (m *Model) IsPage(ctx context.Context, sheet int64) (bool, error) {
	var k int
	err := m.db.Read().QueryRowContext(ctx, `SELECT count(*) FROM _lattis_pages WHERE sheet = ?`, sheet).Scan(&k)
	return k > 0, err
}

// Page donne le contenu d'un onglet page HTML.
func (m *Model) Page(ctx context.Context, sheet int64) (string, error) {
	var src string
	err := m.db.Read().QueryRowContext(ctx, `SELECT source FROM _lattis_pages WHERE sheet = ?`, sheet).Scan(&src)
	if errors.Is(err, sql.ErrNoRows) {
		return "", fmt.Errorf("%w : cette feuille n'est pas une page HTML", ErrSheet)
	}
	return src, err
}

// SetPage remplace le contenu d'un onglet page HTML ; s'annule comme une
// saisie (refusé sur une feuille protégée).
func (m *Model) SetPage(ctx context.Context, sheet int64, source string) error {
	return m.Do(ctx, "Modifier la page HTML", func(t *tx) error {
		var old string
		err := t.row(`SELECT source FROM _lattis_pages WHERE sheet = ?`, sheet).Scan(&old)
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("%w : cette feuille n'est pas une page HTML", ErrSheet)
		}
		if err != nil {
			return err
		}
		if old == source {
			return nil
		}
		if err := t.guard(sheet); err != nil {
			return err
		}
		q := `UPDATE _lattis_pages SET source = ? WHERE sheet = ?`
		return t.apply([]stmt{st(q, source, sheet)}, []stmt{st(q, old, sheet)})
	})
}
