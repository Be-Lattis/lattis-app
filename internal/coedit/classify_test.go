package coedit

import "testing"

func TestDataOnly(t *testing.T) {
	for q, want := range map[string]bool{
		"UPDATE t_1 SET c_2 = ? WHERE id = ?":                                     true,
		"INSERT INTO t_12 (id, _pos) VALUES (?, ?)":                               true,
		"DELETE FROM t_1 WHERE id = ?":                                            true,
		"DELETE FROM _lattis_invalid WHERE tbl = ? AND rid = ? AND col = ?":       true,
		"DELETE FROM _lattis_styles WHERE tbl = ? AND rid = ?":                    true,
		"CREATE TABLE _lattis_trash_sort_1_17 (id INTEGER PRIMARY KEY, pos TEXT)": true,
		"INSERT INTO _lattis_rank_1_17 SELECT id, printf('%012dV', 1) FROM t_1":   true,
		"DROP INDEX t_1_pos":                true,
		"CREATE INDEX t_1_pos ON t_1(_pos)": true,
		"UPDATE t_1 SET _pos = r.k FROM _lattis_rank_1_17 AS r WHERE r.id = t_1.id": true,
		"SELECT 1": true,
		"DELETE FROM _lattis_cells WHERE sheet = ? AND row = ? AND col = ?":            false,
		"INSERT INTO _lattis_cells (sheet, row, col, raw) VALUES (?, ?, ?, ?)":         false,
		"UPDATE _lattis_tables SET name = ? WHERE id = ?":                              false,
		"ALTER TABLE t_1 ADD COLUMN c_4 TEXT":                                          false,
		"DROP TABLE t_1":                                                               false,
		"INSERT INTO _lattis_styles (tbl, rid, style) VALUES (?, ?, ?)":                false,
		"UPDATE t_1_extra SET x = 1":                                                   false,
		"insert into t_3 (id, _pos) values (?, ?)":                                     true,
		"CREATE TABLE _lattis_trash_col_1_17 AS SELECT id, c_2 FROM t_1":               true,
		"INSERT INTO _lattis_trash_col_1_17 SELECT id, c_2 FROM t_1 WHERE c_2 IS NULL": true,
	} {
		if got := dataOnly(q); got != want {
			t.Errorf("%s : %v, attendu %v", q, got, want)
		}
	}
}
