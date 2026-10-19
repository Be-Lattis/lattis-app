package canvas

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/lalamefine/lattis/internal/command"
)

// Tableaux liés à une donnée du Data Lake (requête sur une vue donnees.*
// d'un Serveur Lattis) : après le premier chargement, les relevés du
// serveur (lignes changées, supprimées) s'appliquent ici, comme les
// changements reçus d'un classeur collaboratif (remote.go) : hors du journal
// d'annulation, une ligne étant reconnue par son identifiant dans la donnée
// (_lattis_data_rows), avec le point de reprise dans la même transaction
// (_lattis_data_sync).

var datasetIDs = rowIDs{table: "_lattis_data_rows"}

// ErrDataGap : après un relevé, la copie locale n'a pas autant de lignes
// que la vue du serveur (ligne sortie de l'audience, relevé manqué, ligne
// refusée) ; rien n'est appliqué, il faut tout relire.
var ErrDataGap = errors.New("copie locale de la donnée du Data Lake incomplète : relecture complète")

// DatasetTable décrit le tableau chargé par une requête liée à une donnée.
type DatasetTable struct {
	Table int64
	// Columns : colonnes chargées par la requête (sans les colonnes
	// calculées ajoutées dans le tableau).
	Columns []Column
	// Rows : lignes du tableau ; Bound : celles dont l'identifiant dans la
	// donnée est connu.
	Rows, Bound int
	// Dataset, Since : donnée et point de reprise du dernier relevé ("" :
	// aucun).
	Dataset, Since string
}

// DatasetTable lit le tableau chargé par la requête query ; ok faux si elle
// n'en a pas (pas encore chargée, ou tableau supprimé).
func (m *Model) DatasetTable(ctx context.Context, query int64) (out DatasetTable, ok bool, err error) {
	err = m.read(ctx, func(t *tx) error {
		var target sql.NullInt64
		if err := t.row(`SELECT target FROM _lattis_queries WHERE id = ?`, query).Scan(&target); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return fmt.Errorf("%w introuvable", ErrQuery)
			}
			return err
		}
		if !target.Valid {
			return nil
		}
		tb, err := t.table(target.Int64)
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		if err != nil {
			return err
		}
		ok = true
		out = DatasetTable{Table: tb.ID, Rows: tb.Rows, Columns: []Column{}}
		for _, c := range tb.Cols {
			if c.Formula == "" {
				out.Columns = append(out.Columns, c)
			}
		}
		if err := t.row(`SELECT count(*) FROM _lattis_data_rows WHERE tbl = ?`, tb.ID).Scan(&out.Bound); err != nil {
			return err
		}
		err = t.row(`SELECT dataset, since FROM _lattis_data_sync WHERE tbl = ?`, tb.ID).Scan(&out.Dataset, &out.Since)
		if errors.Is(err, sql.ErrNoRows) {
			err = nil
		}
		return err
	})
	return out, ok, err
}

// DatasetBatch est un relevé d'une donnée à appliquer à son tableau lié.
type DatasetBatch struct {
	Table   int64
	Dataset string
	// Rows : lignes changées (valeurs par colonne locale), ou supprimées
	// (Deleted) ; Full : Rows est toute la donnée, les lignes absentes
	// (et celles dont l'identifiant n'est pas connu, premier chargement par
	// la requête) sont supprimées.
	Rows []RemoteRow
	Full bool
	// Count : lignes de la vue du serveur après ce relevé (-1 : sans
	// contrôle) ; Since : point de reprise noté avec les données.
	Count int
	Since string
}

// ApplyDataset applique un relevé au tableau lié d'une donnée, hors du
// journal d'annulation (comme ApplyRemote). ErrDataGap si le tableau n'a pas
// ensuite le nombre de lignes de la vue : rien n'est appliqué. Un tableau
// supprimé entre-temps est laissé tel quel.
func (m *Model) ApplyDataset(ctx context.Context, b DatasetBatch) (RemoteApplied, error) {
	var out RemoteApplied
	err := m.db.Write(ctx, func(sqlTx *sql.Tx) error {
		out = RemoteApplied{}
		t := &tx{Tx: &command.Tx{Tx: sqlTx, Ctx: ctx}, m: m}
		// Identifiants des tableaux supprimés depuis : oubliés.
		for _, q := range []string{
			`DELETE FROM _lattis_data_rows WHERE tbl NOT IN (SELECT id FROM _lattis_tables)`,
			`DELETE FROM _lattis_data_sync WHERE tbl NOT IN (SELECT id FROM _lattis_tables)`,
		} {
			if _, err := t.ExecContext(ctx, q); err != nil {
				return err
			}
		}
		tb, err := t.table(b.Table)
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		if err != nil {
			return err
		}
		changed := false
		if b.Full {
			// Lignes sans identifiant (premier chargement par la requête, ou
			// tableau rechargé) : remplacées par celles du relevé.
			unbound := ` AND rid NOT IN (SELECT rid FROM _lattis_data_rows WHERE tbl = ` + itoa64(tb.ID) + `)`
			for _, q := range []string{
				`DELETE FROM _lattis_invalid WHERE tbl = ` + itoa64(tb.ID) + unbound,
				`DELETE FROM _lattis_styles WHERE tbl = ` + itoa64(tb.ID) + unbound,
				`DELETE FROM ` + tb.sql() + ` WHERE id NOT IN (SELECT rid FROM _lattis_data_rows WHERE tbl = ` + itoa64(tb.ID) + `)`,
			} {
				res, err := t.ExecContext(ctx, q)
				if err != nil {
					return err
				}
				if n, _ := res.RowsAffected(); n > 0 {
					changed = true
				}
			}
		}
		applied, err := t.applyRemote(datasetIDs, RemoteTable{Table: tb.ID, Rows: b.Rows, Full: b.Full}, nil, &out)
		if err != nil {
			return err
		}
		if b.Count >= 0 {
			var n int
			if err := t.row(`SELECT count(*) FROM ` + tb.sql()).Scan(&n); err != nil {
				return err
			}
			if n != b.Count {
				return ErrDataGap
			}
		}
		if applied || changed {
			out.Tables = append(out.Tables, tb.ID)
		}
		_, err = t.ExecContext(ctx, `INSERT INTO _lattis_data_sync (tbl, dataset, since) VALUES (?, ?, ?)
			ON CONFLICT (tbl) DO UPDATE SET dataset = excluded.dataset, since = excluded.since`, tb.ID, b.Dataset, b.Since)
		return err
	})
	return out, err
}

// ForgetDataset oublie le dernier relevé du tableau tbl (le prochain relit
// toute la donnée).
func (m *Model) ForgetDataset(ctx context.Context, tbl int64) error {
	return m.db.Write(ctx, func(sqlTx *sql.Tx) error {
		_, err := sqlTx.ExecContext(ctx, `DELETE FROM _lattis_data_sync WHERE tbl = ?`, tbl)
		return err
	})
}
