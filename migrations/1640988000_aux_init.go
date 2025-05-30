package migrations

import (
	"fmt"

	"github.com/pocketbase/pocketbase/core"
)

func init() {
	core.SystemMigrations.Add(&core.Migration{
		Up: func(txApp core.App) error {
			switch txApp.AuxDBDriverName() {
			case "sqlite":
				_, execErr := txApp.AuxDB().NewQuery(`
					CREATE TABLE IF NOT EXISTS {{_logs}} (
						[[id]]      TEXT PRIMARY KEY DEFAULT ('r'||lower(hex(randomblob(7)))) NOT NULL,
						[[level]]   INTEGER DEFAULT 0 NOT NULL,
						[[message]] TEXT DEFAULT "" NOT NULL,
						[[data]]    JSON DEFAULT "{}" NOT NULL,
						[[created]] TEXT DEFAULT (strftime('%Y-%m-%d %H:%M:%fZ')) NOT NULL
					);

					CREATE INDEX IF NOT EXISTS idx_logs_level on {{_logs}} ([[level]]);
					CREATE INDEX IF NOT EXISTS idx_logs_message on {{_logs}} ([[message]]);
					CREATE INDEX IF NOT EXISTS idx_logs_created_hour on {{_logs}} (strftime('%Y-%m-%d %H:00:00', [[created]]));
				`).Execute()
				return execErr
			case "pgx":
				if err := createSQLiteEquivalentFunctions(txApp.AuxDB()); err != nil {
					return fmt.Errorf("createSQLiteEquivalentFunctions error: %w", err)
				}
				_, execErr := txApp.AuxDB().NewQuery(`
					CREATE TABLE IF NOT EXISTS {{_logs}} (
						[[id]]      UUID PRIMARY KEY DEFAULT uuid_generate_v7() NOT NULL,
						[[level]]   INTEGER DEFAULT 0 NOT NULL,
						[[message]] TEXT DEFAULT '' NOT NULL,
						[[data]]    JSONB DEFAULT '{}' NOT NULL,
						[[created]] TIMESTAMP DEFAULT now() NOT NULL
					);

					CREATE INDEX IF NOT EXISTS idx_logs_level on {{_logs}} ([[level]]);
					CREATE INDEX IF NOT EXISTS idx_logs_message on {{_logs}} ([[message]]);
					CREATE INDEX IF NOT EXISTS idx_logs_created_hour on {{_logs}} (date_trunc('hour', [[created]]));
				`).Execute()
				return execErr
			}
			panic("Unsupported driver:" + txApp.AuxDBDriverName())
		},
		Down: func(txApp core.App) error {
			_, err := txApp.AuxDB().DropTable("_logs").Execute()
			return err
		},
		ReapplyCondition: func(txApp core.App, runner *core.MigrationsRunner, fileName string) (bool, error) {
			// reapply only if the _logs table doesn't exist
			exists := txApp.AuxHasTable("_logs")
			return !exists, nil
		},
	})
}
