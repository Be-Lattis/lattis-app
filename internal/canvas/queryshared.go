package canvas

import (
	"fmt"
	"strings"

	"github.com/lalamefine/lattis/internal/command"
)

// Actualiser un tableau chargé par une requête et publié dans un classeur
// partagé (audit du mode en ligne, constat 21 ; choix « Actualisé par ce
// poste » à la publication). Remplacer le tableau le supprimerait du serveur
// pour tous (oplog.ErrImplicitDrop) : ses lignes sont remplacées sur place.
// Le résultat est chargé dans un tableau de passage, recopié dans le tableau
// publié par nom de colonne, puis retiré ; le relevé des écritures envoie au
// serveur les lignes supprimées et ajoutées, comme une saisie.

// refreshShared remplace les lignes du tableau publié old par le résultat
// it (voir plus haut), en une étape d'annulation.
func (t *tx) refreshShared(old Table, it ImportTable, res *QueryLoad) error {
	if err := t.guardData(old.Sheet); err != nil {
		return err
	}
	if err := t.lockTable(old.ID); err != nil {
		return err
	}
	// Colonnes du résultat → colonnes saisies du tableau, par leur nom.
	byName := map[string]Column{}
	for _, c := range old.Cols {
		if c.Formula == "" {
			byName[strings.ToLower(c.Name)] = c
		}
	}
	var missing []string
	used := map[int64]bool{}
	for _, h := range it.Headers {
		c, ok := byName[strings.ToLower(h)]
		if !ok {
			missing = append(missing, "« "+h+" »")
			continue
		}
		used[c.ID] = true
	}
	if len(missing) > 0 {
		return fmt.Errorf("%w : la requête rend %s, absente(s) du tableau partagé « %s » ; un concepteur doit d'abord l'ajouter au tableau", ErrQuery, strings.Join(missing, ", "), old.Name)
	}
	for _, c := range old.Cols {
		if c.Formula == "" && !used[c.ID] {
			res.Warnings = append(res.Warnings, fmt.Sprintf("%s : la requête ne rend plus la colonne « %s », laissée vide", old.Name, c.Name))
		}
	}
	// Tableau de passage, loin du contenu de la feuille, retiré ensuite.
	tmp := it
	tmp.At = nil
	tmp.Row, tmp.Col = 1<<20, 0
	name, err := t.freeTableName(old.Name + " (actualisation)")
	if err != nil {
		return err
	}
	tmp.Name = name
	src, err := t.importTable(old.Sheet, &tmp)
	if err != nil {
		return err
	}
	var next int64
	if err := t.row(`SELECT coalesce(max(id), 0) FROM ` + old.sql()).Scan(&next); err != nil {
		return err
	}
	// Lignes copiées dans une table d'attente (identifiants après les
	// anciens, que l'annulation remet) : elle survit au tableau de passage
	// pour un rétablissement, et part avec l'étape (Purge).
	hold := fmt.Sprintf("_lattis_trash_rows_%d_%d_new", old.ID, uniq())
	dst := []string{"id", "_pos"}
	sel := []string{fmt.Sprintf("%d + row_number() OVER (ORDER BY _pos)", next), "_pos"}
	for i, h := range it.Headers {
		c := byName[strings.ToLower(h)]
		dst = append(dst, colSQL(c.ID))
		sel = append(sel, colSQL(src.Cols[i].ID))
	}
	if _, err := t.ExecContext(t.Ctx, `CREATE TABLE `+hold+` AS SELECT `+strings.Join(sel, ", ")+` FROM `+src.sql()); err != nil {
		return err
	}
	if err := t.deleteTable(src); err != nil {
		return err
	}
	if err := t.trashRows(old, `SELECT id FROM `+old.sql()); err != nil {
		return err
	}
	if err := t.ApplyStep(command.Step{
		Do:    []stmt{st(`INSERT INTO ` + old.sql() + ` (` + strings.Join(dst, ", ") + `) SELECT * FROM ` + hold)},
		Undo:  []stmt{st(`DELETE FROM ` + old.sql() + ` WHERE id IN (SELECT id FROM ` + hold + `)`)},
		Purge: []stmt{st(`DROP TABLE IF EXISTS ` + hold)},
	}); err != nil {
		return fmt.Errorf("%w : tableau partagé « %s » : %v", ErrQuery, old.Name, err)
	}
	res.Sheet, res.Table, res.Rows = old.Sheet, old.Name, it.Count
	return nil
}
