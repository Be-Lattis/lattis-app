package workbook

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
)

// Structure d'un tableau d'un classeur collaboratif publié (spécification
// du serveur §9.3, schema/0025_structure.sql, 0026_structure_suite.sql) :
// appels directs, sans point de sauvegarde (sauf insert_column, qui peut
// manquer) ; une erreur défait toute la transaction d'écriture.

func (w pgWriter) RenameTable(ctx context.Context, workbookID, tableID, name string) error {
	_, err := w.tx.Exec(ctx, `SELECT lattis.rename_table($1::uuid, $2::uuid, $3)`, workbookID, tableID, name)
	return err
}

func (w pgWriter) DropTable(ctx context.Context, workbookID, tableID string) error {
	_, err := w.tx.Exec(ctx, `SELECT lattis.drop_table($1::uuid, $2::uuid)`, workbookID, tableID)
	return err
}

func (w pgWriter) AddColumn(ctx context.Context, workbookID, tableID, name, typ, after string, localID int64) (string, error) {
	var id string
	err := w.tx.QueryRow(ctx, `SELECT lattis.add_column($1::uuid, $2::uuid, $3, $4, $5::uuid, $6)::text`,
		workbookID, tableID, name, typ, nullable(after), fmt.Sprint(localID)).Scan(&id)
	return id, err
}

// InsertColumn appelle lattis.insert_column sous un point de sauvegarde :
// absente (42883, serveur d'avant la migration 0026), la transaction est
// ramenée à ce point et reste utilisable (ErrUnsupported).
func (w pgWriter) InsertColumn(ctx context.Context, workbookID, tableID, name, typ string, position int, localID int64) (string, error) {
	const sp = "lattis_insert_column"
	if _, err := w.tx.Exec(ctx, `SAVEPOINT `+sp); err != nil {
		return "", err
	}
	var id string
	err := w.tx.QueryRow(ctx, `SELECT lattis.insert_column($1::uuid, $2::uuid, $3::text, $4::text, $5::integer, $6::text)::text`,
		workbookID, tableID, name, typ, position, fmt.Sprint(localID)).Scan(&id)
	if code, ok := Rejected(err); ok {
		if _, rerr := w.tx.Exec(ctx, `ROLLBACK TO SAVEPOINT `+sp); rerr != nil {
			// La transaction n'est plus utilisable : une coupure, pas un refus.
			return "", fmt.Errorf("retour au point de sauvegarde après « %v » : %s", err, rerr.Error())
		}
		if code == "42883" {
			return "", ErrUnsupported
		}
	}
	if err != nil {
		return "", err
	}
	_, err = w.tx.Exec(ctx, `RELEASE SAVEPOINT `+sp)
	return id, err
}

func (w pgWriter) RenameColumn(ctx context.Context, workbookID, tableID, columnID, name string) error {
	_, err := w.tx.Exec(ctx, `SELECT lattis.rename_column($1::uuid, $2::uuid, $3::uuid, $4)`, workbookID, tableID, columnID, name)
	return err
}

func (w pgWriter) DropColumn(ctx context.Context, workbookID, tableID, columnID string) error {
	_, err := w.tx.Exec(ctx, `SELECT lattis.drop_column($1::uuid, $2::uuid, $3::uuid)`, workbookID, tableID, columnID)
	return err
}

func (w pgWriter) AlterColumnType(ctx context.Context, workbookID, tableID, columnID, typ string) error {
	_, err := w.tx.Exec(ctx, `SELECT lattis.alter_column_type($1::uuid, $2::uuid, $3::uuid, $4)`, workbookID, tableID, columnID, typ)
	return err
}

func (w pgWriter) SetKey(ctx context.Context, workbookID, tableID, columnID string) error {
	_, err := w.tx.Exec(ctx, `SELECT lattis.set_key($1::uuid, $2::uuid, $3::uuid)`, workbookID, tableID, nullable(columnID))
	return err
}

func (w pgWriter) CreateIndex(ctx context.Context, workbookID, tableID string, columnIDs []string, unique bool) (string, error) {
	var name string
	err := w.tx.QueryRow(ctx, `SELECT lattis.create_index($1::uuid, $2::uuid, $3::uuid[], $4)`, workbookID, tableID, columnIDs, unique).Scan(&name)
	return name, err
}

func (w pgWriter) DropIndex(ctx context.Context, workbookID, tableID, name string) error {
	_, err := w.tx.Exec(ctx, `SELECT lattis.drop_index($1::uuid, $2::uuid, $3)`, workbookID, tableID, name)
	return err
}

func (w pgWriter) Indexes(ctx context.Context, workbookID, tableID string) ([]Index, error) {
	var raw []byte
	if err := w.tx.QueryRow(ctx, `SELECT lattis.table_indexes($1::uuid, $2::uuid)::text`, workbookID, tableID).Scan(&raw); err != nil {
		return nil, err
	}
	out := []Index{}
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("lattis.table_indexes : %w", err)
	}
	return out, nil
}

// CreateTable : lattis.create_table sur la version publiée d'un classeur
// collaboratif, comme à la publication (Publisher.CreateTable).
func (w pgWriter) CreateTable(ctx context.Context, workbookID, schema, name string, localID int64, sheet string, cols []ColumnSpec) (string, []RemoteColumn, error) {
	return pgPublisher{tx: w.tx, v9: new(int8)}.CreateTable(ctx, workbookID, schema, name, localID, sheet, cols)
}

// Fill écrit les lignes par INSERT de plusieurs lignes (au plus 60 000
// paramètres par ordre, limite de PostgreSQL : 65 535), envoyés en un lot.
func (w pgWriter) Fill(ctx context.Context, schema, table string, cols []RemoteColumn, rows []NewRow) error {
	if len(rows) == 0 {
		return nil
	}
	names := []string{"lattis_id", "lattis_pos"}
	for _, c := range cols {
		names = append(names, pgx.Identifier{c.Name}.Sanitize())
	}
	head := `INSERT INTO ` + pgx.Identifier{schema, table}.Sanitize() + ` (` + strings.Join(names, ", ") + `) VALUES `
	per := max(1, min(1000, 60000/len(names)))
	b := &pgx.Batch{}
	for start := 0; start < len(rows); start += per {
		part := rows[start:min(len(rows), start+per)]
		var q strings.Builder
		q.WriteString(head)
		args := make([]any, 0, len(part)*len(names))
		for i, r := range part {
			if i > 0 {
				q.WriteString(", ")
			}
			args = append(args, r.ID, r.Pos)
			q.WriteString("($" + strconv.Itoa(len(args)-1) + "::uuid, $" + strconv.Itoa(len(args)))
			for k, c := range cols {
				var v any
				if k < len(r.Vals) {
					var err error
					if v, err = ToRemote(c.Type, r.Vals[k]); err != nil {
						return fmt.Errorf("colonne « %s » : %w", c.Name, err)
					}
				}
				args = append(args, v)
				q.WriteString(", $" + strconv.Itoa(len(args)) + "::" + pgType(c.Type))
			}
			q.WriteString(")")
		}
		b.Queue(q.String(), args...)
	}
	return w.tx.SendBatch(ctx, b).Close()
}

func (w pgWriter) Meta(ctx context.Context, schema, kind, key string) ([]byte, error) {
	return pgReader{tx: w.tx}.Meta(ctx, schema, kind, key)
}

func (w pgWriter) SaveMeta(ctx context.Context, workbookID, kind string, payload []byte) error {
	return pgPublisher{tx: w.tx}.SaveMeta(ctx, workbookID, kind, payload)
}
