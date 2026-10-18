package bridge

import (
	"context"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/lalamefine/lattis/internal/canvas"
	"github.com/lalamefine/lattis/internal/doc"
	"github.com/lalamefine/lattis/internal/grid"
	"github.com/lalamefine/lattis/internal/query"
	"github.com/lalamefine/lattis/internal/remote/workbook"
	"github.com/lalamefine/lattis/protocol"
)

// Relevé d'un tableau lié à une donnée d'entreprise (contrat de lattisd,
// §9 : « comme un tableau collaboratif »). Au lieu de relire toute la
// donnée à chaque notification, seules les lignes changées depuis le
// dernier point de reprise (lattis_xid ≥ since) et les suppressions
// (dataset_tombstones) sont lues, puis appliquées hors du journal
// d'annulation (canvas.ApplyDataset) : une donnée qui change sur le serveur
// ne crée pas d'étape Annuler. Tout est relu, toujours hors du journal,
// quand le point de reprise manque ou n'est plus couvert (pierres tombales
// purgées), quand la vue est refaite (colonnes, audience, republication),
// ou quand la copie locale n'a plus le nombre de lignes de la vue (ligne
// sortie de l'audience). La requête est actualisée comme avant (une étape
// d'annulation) seulement quand le tableau ne peut pas suivre : étapes
// Power Query ou SQL modifiés, colonnes changées de nom ou de type.

// syncDataset relève la donnée ds pour la requête l du classeur d ; full :
// tout relire.
func (s *DocumentService) syncDataset(ctx context.Context, d *doc.Document, srv workbook.Data, l linkedData, ds protocol.Dataset, full bool) error {
	if cur, err := s.current(); err != nil || cur != d {
		return nil
	}
	def, err := d.Model.QueryByID(ctx, l.query)
	if err != nil {
		return err
	}
	q, err := query.Decode(def)
	if err != nil {
		return err
	}
	if !followable(q, ds) {
		_, err := s.RefreshQuery(ctx, l.query)
		return err
	}
	tt, ok, err := d.Model.DatasetTable(ctx, l.query)
	if err != nil || !ok {
		return err // tableau supprimé : plus suivi
	}
	since := tt.Since
	if full || tt.Dataset != ds.ID || tt.Bound != tt.Rows {
		since = ""
	}
	applied, err := s.applyDelta(ctx, d, srv, tt, ds, since)
	if errors.Is(err, canvas.ErrDataGap) && since != "" {
		applied, err = s.applyDelta(ctx, d, srv, tt, ds, "")
	}
	if errors.Is(err, errDataShape) || errors.Is(err, canvas.ErrDataGap) {
		// Colonnes changées, ou valeurs refusées : la requête est
		// actualisée ; le prochain relevé reprend sur le nouveau tableau.
		_, err = s.RefreshQuery(ctx, l.query)
		return err
	}
	if err != nil {
		return err
	}
	if len(applied.Tables) > 0 {
		_, err = s.changed(ctx)
	}
	return err
}

// errDataShape : les colonnes de la donnée ne correspondent plus à celles
// du tableau lié.
var errDataShape = errors.New("colonnes de la donnée changées")

func (s *DocumentService) applyDelta(ctx context.Context, d *doc.Document, srv workbook.Data, tt canvas.DatasetTable, ds protocol.Dataset, since string) (canvas.RemoteApplied, error) {
	delta, err := srv.DatasetDelta(ctx, ds.ID, ds.SQLName, since)
	if err != nil {
		return canvas.RemoteApplied{}, err
	}
	b, err := datasetBatch(tt, ds.ID, delta)
	if err != nil {
		return canvas.RemoteApplied{}, err
	}
	return d.Model.ApplyDataset(ctx, b)
}

// datasetBatch convertit un relevé pour le tableau lié tt : colonnes
// reconnues par leur nom (celles que la requête a chargées, une colonne
// ajoutée à la vue depuis est ignorée), valeurs ramenées au stockage de
// leur colonne comme au premier chargement. errDataShape si une colonne
// manque ou a changé de type.
func datasetBatch(tt canvas.DatasetTable, dataset string, delta workbook.DataDelta) (canvas.DatasetBatch, error) {
	at := map[string]int{}
	for i, c := range delta.Columns {
		at[c] = i
	}
	type pick struct {
		id  int64
		at  int
		typ grid.Type
	}
	picks := make([]pick, 0, len(tt.Columns))
	for _, c := range tt.Columns {
		i, ok := at[c.Name]
		if !ok {
			return canvas.DatasetBatch{}, errDataShape
		}
		if i < len(delta.Types) {
			if want := query.SQLType(delta.Types[i]); want != "" && want != c.Type {
				return canvas.DatasetBatch{}, errDataShape
			}
		}
		picks = append(picks, pick{c.ID, i, c.Type})
	}
	b := canvas.DatasetBatch{Table: tt.Table, Dataset: dataset, Full: delta.Full, Count: delta.Count, Since: delta.Since,
		Rows: make([]canvas.RemoteRow, 0, len(delta.Rows)+len(delta.Deleted))}
	for _, r := range delta.Rows {
		vals := make(map[int64]any, len(picks))
		for _, p := range picks {
			var v any
			if p.at < len(r.Values) {
				v = query.StorageValue(r.Values[p.at], p.typ)
			}
			vals[p.id] = v
		}
		b.Rows = append(b.Rows, canvas.RemoteRow{ID: r.ID, Pos: r.Pos, Values: vals})
	}
	for _, id := range delta.Deleted {
		b.Rows = append(b.Rows, canvas.RemoteRow{ID: id, Deleted: true})
	}
	return b, nil
}

// followable : la requête lit la donnée telle quelle (SQL écrit par
// InsertDataset, sans étape) ; son tableau se tient alors à jour par
// relevés.
func followable(q query.Query, ds protocol.Dataset) bool {
	if q.Source.Kind != query.SourceLattis || len(q.Steps) > 0 {
		return false
	}
	sql := strings.TrimSpace(q.Source.SQL)
	return strings.HasPrefix(sql, "SELECT ") &&
		strings.HasSuffix(sql, " FROM "+protocol.DataSchema+"."+pgx.Identifier{ds.SQLName}.Sanitize()+" ORDER BY lattis_pos")
}
