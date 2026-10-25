package formula

// LIEN_HYPERTEXTE(adresse; [nom]) / HYPERLINK, comme Excel : la valeur de
// la cellule est le nom affiché (l'adresse quand le nom manque), et la
// cellule devient un lien vers l'adresse quand la formule commence par
// cette fonction (=LIEN_HYPERTEXTE(…), pas =SI(…; LIEN_HYPERTEXTE(…))).
// L'adresse se calcule à part (HyperlinkArg) : la grille l'ouvre au clic.

// fnHyperlink : le nom affiché, sinon l'adresse.
func fnHyperlink(a []Value) Value {
	if a[0].IsErr() {
		return a[0]
	}
	if len(a) > 1 && !a[1].IsEmpty() {
		return a[1]
	}
	return a[0]
}

// HyperlinkArg : l'expression de l'adresse d'une formule qui commence par
// LIEN_HYPERTEXTE (faux sinon).
func HyperlinkArg(n Node) (Node, bool) {
	c, ok := n.(Call)
	if !ok || c.Fn != "HYPERLINK" || len(c.Args) == 0 {
		return nil, false
	}
	return c.Args[0], true
}

// HyperlinkTarget : pour une formule de cellule (raw, en français) qui
// commence par LIEN_HYPERTEXTE, la formule de son adresse (« ="https://…"&A1 »),
// à évaluer comme la cellule ; "" sinon.
func HyperlinkTarget(raw string) string {
	n, err := Parse(raw, FR)
	if err != nil {
		return ""
	}
	a, ok := HyperlinkArg(n)
	if !ok {
		return ""
	}
	return Format(a, FR)
}
