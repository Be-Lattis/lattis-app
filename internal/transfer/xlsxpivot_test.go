package transfer

import (
	"context"
	"strings"
	"testing"
)

// Un tableau croisé dont la source est une plage du classeur devient un
// tableau croisé de Lattis (lignes, colonnes, valeurs, filtre de rapport) ;
// ses cellules ne sont pas importées en plus comme valeurs.
func TestXLSXImportPivot(t *testing.T) {
	ctx := context.Background()
	data := `<sheetData>` +
		row(1, is("A1", "Région"), is("B1", "Produit"), is("C1", "Ventes"), is("D1", "Année")) +
		row(2, is("A2", "Nord"), is("B2", "Pomme"), cn("C2", "10"), cn("D2", "2025")) +
		row(3, is("A3", "Sud"), is("B3", "Poire"), cn("C3", "20"), cn("D3", "2025")) +
		row(4, is("A4", "Nord"), is("B4", "Poire"), cn("C4", "5"), cn("D4", "2026")) +
		`</sheetData>`
	// Rendu d'Excel : filtre en B1, en-têtes en A3, lignes, total.
	tcd := `<sheetData>` +
		row(1, is("A1", "Année"), cn("B1", "2025")) +
		row(3, is("A3", "Étiquettes de lignes"), is("B3", "Moyenne de Ventes"), is("C3", "Somme de Ventes")) +
		row(4, is("A4", "Nord"), cn("B4", "10"), cn("C4", "10")) +
		row(5, is("A5", "Sud"), cn("B5", "20"), cn("C5", "20")) +
		row(6, is("A6", "Total général"), cn("B6", "15"), cn("C6", "30")) +
		`</sheetData>`
	parts := map[string]string{
		"xl/worksheets/_rels/sheet2.xml.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="p1" Type="` + relNS + `pivotTable" Target="../pivotTables/pivotTable1.xml"/></Relationships>`,
		"xl/pivotTables/_rels/pivotTable1.xml.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="c1" Type="` + relNS + `pivotCacheDefinition" Target="../pivotCache/pivotCacheDefinition1.xml"/></Relationships>`,
		"xl/pivotTables/pivotTable1.xml": `<pivotTableDefinition ` + xmlNS + ` name="TCD1" cacheId="1"><location ref="A3:C6" firstHeaderRow="0" firstDataRow="1" firstDataCol="1" rowPageCount="1" colPageCount="1"/>` +
			`<pivotFields count="4"><pivotField axis="axisRow"><items count="3"><item x="0"/><item x="1"/><item t="default"/></items></pivotField><pivotField/><pivotField dataField="1"/>` +
			`<pivotField axis="axisPage"><items count="3"><item x="1"/><item x="0"/><item t="default"/></items></pivotField></pivotFields>` +
			`<rowFields count="1"><field x="0"/></rowFields><colFields count="1"><field x="-2"/></colFields>` +
			`<pageFields count="1"><pageField fld="3" item="1" hier="-1"/></pageFields>` +
			`<dataFields count="2"><dataField name="Moyenne de Ventes" fld="2" subtotal="average"/><dataField name="Total des ventes" fld="2"/></dataFields></pivotTableDefinition>`,
		"xl/pivotCache/pivotCacheDefinition1.xml": `<pivotCacheDefinition ` + xmlNS + `><cacheSource type="worksheet"><worksheetSource ref="A1:D4" sheet="Données"/></cacheSource>` +
			`<cacheFields count="4"><cacheField name="Région"><sharedItems><s v="Nord"/><s v="Sud"/></sharedItems></cacheField><cacheField name="Produit"><sharedItems/></cacheField>` +
			`<cacheField name="Ventes"><sharedItems containsNumber="1"/></cacheField><cacheField name="Année"><sharedItems><n v="2025"/><n v="2026"/></sharedItems></cacheField></cacheFields></pivotCacheDefinition>`,
	}
	path := bookParts(t, [][2]string{{"Données", data}, {"TCD", tcd}}, styleSheet, parts)
	d := open(t)
	res, err := ImportXLSX(ctx, d.Model, path)
	if err != nil {
		t.Fatal(err)
	}
	check(t, d, "après import")
	if w := strings.Join(res.Warnings, "\n"); strings.Contains(w, "croisé") {
		t.Errorf("avertissements : %s", w)
	}
	l, err := d.Layout(ctx, res.Sheets[1])
	if err != nil {
		t.Fatal(err)
	}
	if len(l.Tables) != 0 || len(l.Cells) != 0 {
		t.Errorf("cellules du TCD importées en plus : %d tableaux, %d cellules", len(l.Tables), len(l.Cells))
	}
	if len(l.Pivots) != 1 {
		t.Fatalf("%d TCD", len(l.Pivots))
	}
	p := l.Pivots[0]
	c := p.Config
	if p.Row != 0 || p.Col != 0 || c.Source != "Données" || strings.Join(c.RowFields, ",") != "Région" || len(c.Values) != 2 ||
		c.Values[0].Agg != "avg" || c.Values[0].Label != "" || c.Values[1].Agg != "sum" || c.Values[1].Label != "Total des ventes" ||
		len(c.Filters) != 1 || c.Filters[0].Field != "Année" || strings.Join(c.Filters[0].Values, ",") != "2025" {
		t.Errorf("TCD : %+v à (%d, %d)", c, p.Row, p.Col)
	}
	if p.Error != "" || len(p.Cells) < 3 || p.Cells[1][0] != "Nord" {
		t.Errorf("résultat : %v %s", p.Cells, p.Error)
	}
}
