package workbook

import (
	"context"
	"database/sql"
)

// Clés de _lattis_info propres à la synchronisation d'un classeur
// collaboratif (internal/remote/collab : skipKey, probeKey).
const (
	syncSkipKey  = "sync_skip"
	syncProbeKey = "sync_probe"
)

// Detach retire d'un classeur tout ce qui le relie à son serveur
// (« Détacher du serveur », « Travailler sur une copie locale ») : la
// description du classeur partagé et sa mise en page de référence
// (_lattis_info), la correspondance des tableaux publiés, les identifiants
// partagés des lignes, la file d'envoi, les conflits et les droits connus.
// Les données et la mise en page restent telles qu'elles sont sur ce poste,
// modifications pas encore envoyées comprises : le classeur devient un
// classeur local ordinaire (le manifeste n'a plus de section remote au
// prochain enregistrement). Le classeur du serveur n'est pas touché.
//
// Les tableaux liés à une donnée du Data Lake (_lattis_data_rows,
// _lattis_data_sync) ne relèvent pas du partage du classeur : ils restent
// liés.
func Detach(ctx context.Context, tx *sql.Tx) error {
	if _, err := tx.ExecContext(ctx, `DELETE FROM _lattis_info WHERE key IN (?, ?, ?, ?)`, infoKey, baseKey, syncSkipKey, syncProbeKey); err != nil {
		return err
	}
	for _, t := range []string{"_lattis_remote_tables", "_lattis_row_ids", "_lattis_outbox", "_lattis_conflicts", "_lattis_capabilities"} {
		if _, err := tx.ExecContext(ctx, `DELETE FROM `+t); err != nil {
			return err
		}
	}
	return nil
}

// Unsent compte les étapes d'un classeur collaboratif pas encore envoyées
// au serveur (file d'envoi) et les conflits ouverts.
func Unsent(ctx context.Context, q queryer) (pending, conflicts int, err error) {
	err = q.QueryRowContext(ctx, `SELECT (SELECT count(DISTINCT batch) FROM _lattis_outbox WHERE state = 'pending'), (SELECT count(*) FROM _lattis_conflicts)`).Scan(&pending, &conflicts)
	return
}
