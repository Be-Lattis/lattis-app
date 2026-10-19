package workbook

import (
	"context"
	"encoding/json"
	"fmt"
)

// Structure d'un tableau d'un classeur collaboratif publié (spécification
// du serveur §9.3, schema/0025_structure.sql) : appels directs, sans point
// de sauvegarde ; une erreur défait toute la transaction d'écriture.

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
