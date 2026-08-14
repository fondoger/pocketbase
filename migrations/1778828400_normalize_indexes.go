package migrations

import (
	"fmt"
	"strings"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/dbutils"
)

// see https://github.com/pocketbase/pocketbase/issues/7689
func init() {
	core.SystemMigrations.Register(func(txApp core.App) error {
		collections, err := txApp.FindAllCollections()
		if err != nil {
			return err
		}

		for _, collection := range collections {
			// existing system collection indexes can't be modified and view don't have indexes
			if collection.System || collection.IsView() {
				continue
			}

			masterIndexes := []struct {
				Name string `db:"name"`
				SQL  string `db:"sql"`
			}{}

			err := txApp.DB().Select("name", "sql").
				From("sqlite_master").
				AndWhere(dbx.HashExp{
					"type":     "index",
					"tbl_name": collection.Name,
				}).
				AndWhere(dbx.NewExp("sql IS NOT NULL AND name NOT LIKE 'sqlite_autoindex_%'")).
				All(&masterIndexes)
			if err != nil {
				return err
			}

			// no indexes
			if len(masterIndexes) == 0 && len(collection.Indexes) == 0 {
				continue
			}

			missingParsedIndexes := map[string]dbutils.Index{}

			// find missing master indexes
		masterLoop:
			for _, masterIndex := range masterIndexes {
				mParsed := dbutils.ParseIndex(masterIndex.SQL)
				mParsed.SchemaName = ""
				mParsed.TableName = collection.Name

				for _, raw := range collection.Indexes {
					cParsed := dbutils.ParseIndex(raw)

					// index already exists (if needed it will be normalized on resave)
					if cParsed.IndexName != "" && strings.EqualFold(cParsed.IndexName, mParsed.IndexName) {
						continue masterLoop
					}
				}

				missingParsedIndexes[masterIndex.Name] = mParsed
			}

		missingIndexesLoop:
			for _, missing := range missingParsedIndexes {
				missingSQL := missing.Build()

				// it shouldn't be possible but for just in case if there is an edge case the regex doesn't cover
				if missingSQL == "" {
					return fmt.Errorf("failed to build sqlite_master index: %v", missing)
				}

				// drop the missing index to recreate later
				_, err := txApp.DB().DropIndex(missing.TableName, missing.IndexName).Execute()
				if err != nil {
					return fmt.Errorf("failed to drop index %s: %w", missing.IndexName, err)
				}

				// no recreate: duplicated single unique tokenKey or email
				// (auth collections are guaranteed to have them)
				if collection.IsAuth() && missing.Unique && len(missing.Columns) == 1 &&
					(strings.EqualFold(missing.Columns[0].Name, core.FieldNameTokenKey) || strings.EqualFold(missing.Columns[0].Name, core.FieldNameEmail)) {
					continue missingIndexesLoop
				}

				// no recreate: the same index definition alreay exists
				// in the collection but with different name
				for _, raw := range collection.Indexes {
					cParsed := dbutils.ParseIndex(raw)
					cParsed.IndexName = missing.IndexName
					cParsed.SchemaName = missing.SchemaName
					cParsed.TableName = missing.TableName
					cSQL := cParsed.Build()

					if missingSQL == cSQL {
						continue missingIndexesLoop
					}
				}

				// recreate: add the missing index to the collection list and
				// leave the user to decide whether they want to keep it or not
				// (the index could have been previously created externally, e.g. via the sqlite3 cli)
				collection.Indexes = append(collection.Indexes, missingSQL)
			}

			// resave to trigger indexes normalization
			err = txApp.Save(collection)
			if err != nil {
				return err
			}
		}

		return nil
	}, nil)
}
