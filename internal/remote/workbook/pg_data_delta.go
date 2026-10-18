package workbook

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/lalamefine/lattis/protocol"
)

// Relevé incrémental d'un tableau lié (contrat client de lattisd, §9) et
// requêtes de l'onglet SQL sur les vues donnees.*.

// dataSystem : colonnes système des vues donnees.*, jamais montrées.
var dataSystem = map[string]bool{"lattis_id": true, "lattis_pos": true, "lattis_xid": true, "lattis_updated_at": true, "lattis_author": true, "lattis_updated_by": true}

func (s pgServer) DatasetDelta(ctx context.Context, id, sqlName, since string) (DataDelta, error) {
	var out DataDelta
	view := protocol.DataSchema + "." + pgx.Identifier{sqlName}.Sanitize()
	err := s.with(ctx, usageLoad, func(c *pgx.Conn) error {
		return pgx.BeginTxFunc(ctx, c, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly}, func(tx pgx.Tx) error {
			out = DataDelta{}
			// Point de reprise pris avant la lecture : une transaction validée
			// après lui sera relue la prochaine fois.
			if err := tx.QueryRow(ctx, `SELECT lattis.sync_point()::text`).Scan(&out.Since); err != nil {
				return err
			}
			out.Full = since == ""
			if !out.Full {
				var covered bool
				if err := tx.QueryRow(ctx, `SELECT lattis.since_covered($1::xid8)`, since).Scan(&covered); err != nil {
					return err
				}
				out.Full = !covered
			}
			q, args := `SELECT * FROM `+view, []any{}
			if !out.Full {
				// ≥ : sync_point est le plus petit xid en cours ; une ligne
				// revenue deux fois s'applique deux fois sans effet.
				q += ` WHERE lattis_xid >= $1::xid8`
				args = append(args, since)
			}
			rows, err := tx.Query(ctx, q+` ORDER BY lattis_pos, lattis_id`, args...)
			if err != nil {
				return err
			}
			defer rows.Close()
			idCol, posCol := -1, -1
			var keep []int
			for i, f := range rows.FieldDescriptions() {
				switch {
				case f.Name == "lattis_id":
					idCol = i
				case f.Name == "lattis_pos":
					posCol = i
				case dataSystem[f.Name]:
				default:
					keep = append(keep, i)
					out.Columns = append(out.Columns, f.Name)
					name := ""
					if t, ok := c.TypeMap().TypeForOID(f.DataTypeOID); ok {
						name = strings.ToUpper(t.Name)
					}
					out.Types = append(out.Types, name)
				}
			}
			for rows.Next() {
				vals, err := rows.Values()
				if err != nil {
					return err
				}
				r := DataRow{Values: make([]any, len(keep))}
				for j, i := range keep {
					r.Values[j] = dataValue(vals[i])
				}
				if idCol >= 0 {
					r.ID = dataText(vals[idCol])
				}
				if posCol >= 0 {
					r.Pos = dataText(vals[posCol])
				}
				out.Rows = append(out.Rows, r)
			}
			if err := rows.Err(); err != nil {
				return err
			}
			if out.Full {
				out.Count = len(out.Rows)
				return nil
			}
			// dataset_tombstones prend les suppressions d'xid > p_since : depuis
			// since compris, comme les lignes (voir xidBefore).
			ts, err := tx.Query(ctx, `SELECT row_id::text FROM lattis.dataset_tombstones($1::uuid, $2::xid8)`, id, xidBefore(since))
			if err != nil {
				return err
			}
			for ts.Next() {
				var rid string
				if err := ts.Scan(&rid); err != nil {
					ts.Close()
					return err
				}
				out.Deleted = append(out.Deleted, rid)
			}
			ts.Close()
			if err := ts.Err(); err != nil {
				return err
			}
			return tx.QueryRow(ctx, `SELECT count(*) FROM `+view).Scan(&out.Count)
		})
	})
	return out, err
}

// xidBefore rend l'xid8 qui précède x ("0" pour 0 ou une valeur illisible).
func xidBefore(x string) string {
	n, err := strconv.ParseUint(x, 10, 64)
	if err != nil || n == 0 {
		return "0"
	}
	return strconv.FormatUint(n-1, 10)
}

// dataValue ramène une valeur lue par pgx à ce que rend database/sql avec le
// pilote pgx (stdlib) : la conversion locale est alors celle du premier
// chargement de la requête (query.StorageValue).
func dataValue(v any) any {
	switch x := v.(type) {
	case nil, string, bool, int64, float64, time.Time:
		return v
	case int32:
		return int64(x)
	case int16:
		return int64(x)
	case int8:
		return int64(x)
	case int:
		return int64(x)
	case float32:
		return float64(x)
	case []byte:
		return string(x)
	case pgtype.Numeric:
		if !x.Valid {
			return nil
		}
	}
	return dataText(v)
}

func (s pgServer) DataQuery(ctx context.Context, q string, limit int) (DataResult, error) {
	out := DataResult{Columns: []string{}, Rows: [][]any{}}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	err := s.with(ctx, usageLoad, func(c *pgx.Conn) error {
		return pgx.BeginTxFunc(ctx, c, pgx.TxOptions{AccessMode: pgx.ReadOnly}, func(tx pgx.Tx) error {
			out = DataResult{Columns: []string{}, Rows: [][]any{}}
			// Protocole étendu : une seule instruction ; la transaction en
			// lecture seule est défaite à la fin.
			rows, err := tx.Query(ctx, q)
			if err != nil {
				return err
			}
			defer rows.Close()
			for _, f := range rows.FieldDescriptions() {
				out.Columns = append(out.Columns, f.Name)
			}
			for rows.Next() {
				if limit > 0 && len(out.Rows) == limit {
					out.Truncated = true
					break
				}
				vals, err := rows.Values()
				if err != nil {
					return err
				}
				row := make([]any, len(vals))
				for i, v := range vals {
					row[i] = displayValue(v)
				}
				out.Rows = append(out.Rows, row)
			}
			return rows.Err()
		})
	})
	return out, err
}

// displayValue : une valeur de PostgreSQL pour la grille de résultat de
// l'onglet SQL (nombres en nombres, le reste en texte comme une saisie).
func displayValue(v any) any {
	switch x := v.(type) {
	case nil:
		return nil
	case int64, float64:
		return v
	case int32, int16, int8:
		return dataValue(v)
	case float32:
		return float64(x)
	case pgtype.Numeric:
		if f, err := x.Float64Value(); err == nil && f.Valid {
			return f.Float64
		}
	case string:
		return x
	}
	if s := dataText(v); s != "" {
		return s
	}
	return fmt.Sprint(v)
}
