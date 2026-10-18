package integration

import (
	"strings"
	"testing"
)

// circ vérifie que toute la colonne est en référence circulaire.
func (h *host) wantCirc(table, col string) {
	h.t.Helper()
	for _, v := range h.column(table, col) {
		if v != "#CIRC!" {
			h.t.Fatalf("%s[%s] = %v, attendu #CIRC! partout", table, col, h.column(table, col))
		}
	}
}

// Audit n° 3, C1 : une colonne calculée qui agrège sa propre colonne
// (forme qualifiée) faisait déborder la pile et arrêtait l'application, à
// la saisie comme à la récupération. C'est une référence circulaire (B1).
func TestSelfAggregateColumn(t *testing.T) {
	h := newHost(t)
	h.paste(1, 1, []any{"Produit", "Prix", "X"}, []any{"a", 1, 2}, []any{"b", 3, 4})
	h.onlyTable("Feuille1")
	h.set(2, 3, "=SOMME(Feuille1[X])")
	h.wantCirc("Feuille1", "X")
	// Une cellule libre qui lit la colonne reçoit l'erreur.
	h.set(6, 1, "=SOMME(Feuille1[X])")
	h.wantVal(6, 1, "#CIRC!")
	// Forme non qualifiée : même erreur (et non plus #REF!).
	h.set(2, 3, "=SOMME([X])")
	h.wantCirc("Feuille1", "X")
	// À travers une autre colonne, et par MOYENNE ou NB.SI (précalculés).
	h.set(2, 3, "=[Prix]*2")
	h.set(1, 4, "Y")
	h.set(2, 4, "=MOYENNE(Feuille1[X])+NB.SI(Feuille1[Y];1)")
	h.wantCirc("Feuille1", "Y")
	h.wantColumn("Feuille1", "X", "2", "6")
	h.set(2, 3, "=[Y]+1")
	h.wantCirc("Feuille1", "X")
	// Casser la boucle rétablit les valeurs.
	h.set(2, 4, "=[Prix]")
	h.wantColumn("Feuille1", "X", "2", "4")

	// Le classeur s'enregistre et se rouvre avec la boucle : l'erreur reste
	// dans les cellules.
	h.set(2, 3, "=SOMME(Feuille1[X])")
	h.reopen(path(t, "boucle.lattis"))
	h.wantCirc("Feuille1", "X")
	h.wantVal(6, 1, "#CIRC!")
}

// Les autres chemins vers la même boucle : un total saisi juste sous le
// tableau (la ligne est absorbée), et une formule libre absorbée quand le
// tableau s'élargit jusqu'à elle.
func TestSelfAggregateAbsorbed(t *testing.T) {
	h := newHost(t)
	h.paste(1, 1, []any{"Produit", "X"}, []any{"a", 2}, []any{"b", 4})
	h.onlyTable("T")
	h.set(4, 2, "=SOMME(T[X])")
	if tb := h.table("T"); tb.Rows != 3 {
		t.Fatalf("le total saisi sous le tableau n'a pas été absorbé : %+v", tb)
	}
	h.wantCirc("T", "X")

	h = newHost(t)
	h.paste(1, 1, []any{"Produit", "X"}, []any{"a", 2}, []any{"b", 4})
	h.onlyTable("T")
	h.set(2, 3, "=SOMME(T[Z])") // colonne encore absente : #REF!
	h.wantVal(2, 3, "#REF!")
	h.set(1, 3, "Z") // le tableau s'élargit et absorbe la formule
	if cols := h.table("T").Columns; len(cols) != 3 || cols[2].Formula == "" {
		t.Fatalf("formule non absorbée : %+v", cols)
	}
	h.wantCirc("T", "Z")
}

// Une colonne qui lit ses propres lignes voisines n'est pas circulaire :
// [Solde][-1], PRECEDENT([Solde]) et [Reste][+1] se calculent ligne après
// ligne ; lire sa propre ligne, ou dans les deux sens, reste circulaire.
func TestSelfNeighbourColumns(t *testing.T) {
	h := newHost(t)
	h.paste(1, 1, []any{"Mvt"}, []any{1}, []any{2}, []any{5}, []any{10})
	h.onlyTable("T")
	h.set(1, 2, "Solde")
	h.set(2, 2, "=[Mvt]+PRECEDENT([Solde]; 100)")
	h.wantColumn("T", "Solde", "101", "103", "108", "118")
	h.set(1, 3, "Reste")
	h.set(2, 3, "=[Mvt]+[Reste][+1]")
	h.wantColumn("T", "Reste", "18", "17", "15", "10")
	h.set(1, 4, "Deux")
	h.set(2, 4, "=[Mvt]+[Deux][-2]")
	h.wantColumn("T", "Deux", "1", "2", "6", "12")
	// Lu ailleurs : agrégat et cellule libre.
	h.set(8, 1, "=SOMME(T[Solde])")
	h.wantVal(8, 1, "430")
	// Un tri recalcule dans le nouvel ordre.
	h.do("tables.sort", "table", "T", "column", "Mvt", "descending", true)
	h.wantColumn("T", "Solde", "110", "115", "117", "118")
	h.do("tables.sort", "table", "T", "column", "Mvt")
	// Une erreur se propage aux lignes suivantes, pas aux précédentes.
	h.set(1, 5, "Inv")
	h.set(2, 5, "=1/([Mvt]-5)")
	h.set(2, 2, "=[Inv]+PRECEDENT([Solde]; 0)")
	if got := strings.Join(h.column("T", "Solde"), "|"); got != "-0.25|-0.5833333333333333|#DIV/0!|#DIV/0!" {
		t.Fatalf("Solde après une erreur : %s", got)
	}
	h.set(2, 4, "=[Mvt]+[Deux][-1]+[Deux][+1]")
	h.wantCirc("T", "Deux")
	h.set(2, 4, "=[Mvt]+[Deux]")
	h.wantCirc("T", "Deux")
}

// Une cellule vide d'une autre feuille vaut vide ; une feuille inconnue
// donne #NOM?.
func TestEmptyCellOtherSheet(t *testing.T) {
	h := newHost(t)
	h.do("sheets.add", "name", "Bilan")
	h.set(1, 1, "=Bilan!Cell[C3]*2+1")
	h.wantVal(1, 1, "1")
	h.set(2, 1, "=Inconnue!Cell[C3]+1")
	h.wantVal(2, 1, "#NOM?")
}
