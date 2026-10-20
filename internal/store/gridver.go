package store

import (
	"github.com/ncruces/go-sqlite3"
)

// Suivi des écritures par ligne (sqlite3 update hook), en plus du texte des
// requêtes (noteWrite) : le texte ne voit pas les lignes écrites par un
// déclencheur (vues de l'onglet SQL), une requête qui écrit sans nommer
// t_N, ni une colonne de _lattis_sheets autre que dans « SET choice ».
// Une grille gardée en cache sur un tableau dont les lignes avaient changé
// lisait des lignes en trop, en moins, ou « grid: hors du tableau ».

// rowKey : une opération sur une table, relevée une fois par tâche.
type rowKey struct {
	op    sqlite3.AuthorizerActionCode
	table string
}

// gridMetaTables : tables de métadonnées dont une ligne écrite peut
// changer ce que lit un tableau de la grille (colonnes et formules,
// tableaux, filtre, suivi de l'onglet, saisies invalides, droits d'un
// classeur collaboratif).
var gridMetaTables = map[string]bool{
	"_lattis_tables":       true,
	"_lattis_columns":      true,
	"_lattis_invalid":      true,
	"_lattis_capabilities": true,
}

// cellTables : valeurs hors des tableaux, que peuvent lire les colonnes
// calculées (cellules libres, cellules par valeur de l'onglet, noms).
var cellTables = map[string]bool{
	"_lattis_cells":        true,
	"_lattis_choice_cells": true,
	"_lattis_names":        true,
	"_lattis_range_cells":  true,
}

// isDataTable : t_N, la table d'un tableau du classeur.
func isDataTable(name string) bool {
	if len(name) < 3 || name[:2] != "t_" {
		return false
	}
	for _, c := range name[2:] {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

// noteRow relève une ligne insérée, modifiée ou supprimée par l'écrivain.
// Appelé par SQLite pour chaque ligne : une seule prise de verrou par
// (opération, table) et par tâche.
func (s *Store) noteRow(op sqlite3.AuthorizerActionCode, schema, table string) {
	if schema != "main" {
		return
	}
	if w := s.watched.Load(); w != nil && (*w)[table] {
		s.watchGen.Add(1)
	}
	data, meta, cells := isDataTable(table), gridMetaTables[table], cellTables[table]
	sheets := table == "_lattis_sheets" && op != sqlite3.AUTH_UPDATE
	if !data && !meta && !cells && !sheets {
		return
	}
	// seen n'est lu et écrit que par la goroutine de l'écrivain (ici et
	// dans settle) : pas de verrou pour les lignes suivantes.
	k := rowKey{op, table}
	if s.seen[k] {
		return
	}
	if s.seen == nil {
		s.seen = map[rowKey]bool{}
	}
	s.seen[k] = true
	s.tmu.Lock()
	defer s.tmu.Unlock()
	switch {
	case data:
		if s.tgen == nil {
			s.tgen = map[string]int64{}
		}
		if s.pending == nil {
			s.pending, s.pendingRows, s.sgen = map[string]bool{}, map[string]bool{}, map[string]int64{}
		}
		if s.content == nil {
			s.content, s.structure = map[string]bool{}, map[string]bool{}
		}
		s.tgen[table]++
		s.pending[table] = true
		s.dataGen++
		s.pendingData = true
		// Une mise à jour peut aussi déplacer la ligne (_pos) : le texte de
		// la requête le dit (noteWrite) ; ici, seulement des valeurs.
		if op == sqlite3.AUTH_UPDATE {
			s.content[table] = true
			return
		}
		s.sgen[table]++
		s.pendingRows[table] = true
		s.structure[table] = true
	case cells:
		s.dataGen++
		s.pendingData = true
		s.pendingCells = true
	default: // métadonnées, feuille ajoutée ou retirée
		s.touchedAll = true
		s.gridGen++
		s.pendingGrid = true
		if table != "_lattis_invalid" && table != "_lattis_capabilities" {
			s.metaGen++
			s.pendingMeta = true
		}
	}
}

// GridVersion donne les versions de ce que lit un tableau de la grille dont
// la source SQL lit les tables tables (t_N) : rows avance quand ses lignes
// (ajouts, suppressions, ordre) ou sa définition (colonnes, filtre, choix
// de l'onglet, suivi) ont pu changer ; values quand des valeurs de ces
// tables ont pu changer ; data à chaque écriture de données du classeur
// (ce que lisent les colonnes calculées). Comme Version, une écriture en
// cours fait encore avancer les versions à sa fin : un tableau lu pendant
// l'écriture ne sert plus après.
func (s *Store) GridVersion(tables []string) (rows, values, data int64) {
	s.tmu.Lock()
	defer s.tmu.Unlock()
	rows = s.gridGen
	for _, n := range tables {
		rows += s.sgen[n]
		values += s.tgen[n]
	}
	return rows, values, s.dataGen
}
