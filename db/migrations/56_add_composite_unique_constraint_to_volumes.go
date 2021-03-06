package migrations

import "github.com/BurntSushi/migration"

func AddCompositeUniqueConstraintToVolumes(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
    ALTER TABLE volumes DROP CONSTRAINT volumes_handle_key;
    ALTER TABLE volumes ADD UNIQUE (worker_name, handle);
	`)

	return err
}
