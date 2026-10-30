package transfer

// Îlots sans ligne d'en-têtes : un îlot de plusieurs lignes devient un
// tableau, sa première ligne donnant les en-têtes. Une première ligne qui
// porte une formule n'est pas une ligne d'en-têtes (Libellé | =SOMME(…)) :
// l'îlot reste en cellules libres, la formule garde sa place. Pour un
// fichier .xlsx ouvert dans Lattis (keep), un nombre, une date ou un
// booléen en première ligne d'un petit îlot aussi : un en-tête est un texte,
// et la cellule doit se réécrire telle quelle dans le fichier.

const (
	maxHeadCells = 200_000 // cellules notées (feuilles énormes : au-delà, rien)
	maxFreeRows  = 500     // îlot gardé en cellules libres pour un nombre
)

// headerCells note les cellules qui ne peuvent pas être un en-tête.
type headerCells struct {
	formula, value map[[2]int]bool
}

func (h *headerCells) note(x *xlsxFile, c *rawCell) {
	text := c.t == "s" || c.t == "inlineStr" || c.t == "str"
	switch {
	case c.hasF:
		if h.formula == nil {
			h.formula = map[[2]int]bool{}
		}
		if len(h.formula) < maxHeadCells {
			h.formula[[2]int{c.row, c.col}] = true
		}
	case x.keep && !text:
		if h.value == nil {
			h.value = map[[2]int]bool{}
		}
		if len(h.value) < maxHeadCells {
			h.value[[2]int{c.row, c.col}] = true
		}
	}
}

// free dit si l'îlot r reste en cellules libres.
func (h *headerCells) free(x *xlsxFile, r rect) bool {
	if r.r1 == r.r0 {
		return false
	}
	small := r.r1-r.r0 <= maxFreeRows
	for c := r.c0; c <= r.c1; c++ {
		k := [2]int{r.r0, c}
		if h.formula[k] || small && h.value[k] {
			return true
		}
	}
	return false
}
