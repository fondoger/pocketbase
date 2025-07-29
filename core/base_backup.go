package core

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"time"

	"github.com/pocketbase/pocketbase/tools/archive"
	"github.com/pocketbase/pocketbase/tools/filesystem"
	"github.com/pocketbase/pocketbase/tools/inflector"
	"github.com/pocketbase/pocketbase/tools/osutils"
	"github.com/pocketbase/pocketbase/tools/security"
)

const (
	StoreKeyActiveBackup = "@activeBackup"
)

// PostgresConnectionInfo holds the parsed connection details
type PostgresConnectionInfo struct {
	Host     string
	Port     string
	User     string
	Password string
	Database string
	SSLMode  string
}

// copyDir copies a directory recursively
func copyDir(src string, dest string) error {
	if err := os.MkdirAll(dest, os.ModePerm); err != nil {
		return err
	}

	sourceDir, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceDir.Close()

	items, err := sourceDir.Readdir(-1)
	if err != nil {
		return err
	}

	for _, item := range items {
		fullSrcPath := filepath.Join(src, item.Name())
		fullDestPath := filepath.Join(dest, item.Name())

		var copyErr error
		if item.IsDir() {
			copyErr = copyDir(fullSrcPath, fullDestPath)
		} else {
			copyErr = copyFile(fullSrcPath, fullDestPath)
		}

		if copyErr != nil {
			return copyErr
		}
	}

	return nil
}

// copyFile copies a single file
func copyFile(src string, dest string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	destFile, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer destFile.Close()

	if _, err := io.Copy(destFile, srcFile); err != nil {
		return err
	}

	return nil
}

// parsePostgresURL parses a PostgreSQL connection URL and extracts connection parameters
func parsePostgresURL(connectionURL string) (*PostgresConnectionInfo, error) {
	u, err := url.Parse(connectionURL)
	if err != nil {
		return nil, fmt.Errorf("invalid connection URL: %w", err)
	}

	info := &PostgresConnectionInfo{
		Host:     u.Hostname(),
		Port:     u.Port(),
		Database: u.Path[1:], // Remove leading slash
		SSLMode:  "disable",  // Default
	}

	if info.Port == "" {
		info.Port = "5432" // Default PostgreSQL port
	}

	if u.User != nil {
		info.User = u.User.Username()
		if password, ok := u.User.Password(); ok {
			info.Password = password
		}
	}

	// Parse query parameters
	if sslmode := u.Query().Get("sslmode"); sslmode != "" {
		info.SSLMode = sslmode
	}

	return info, nil
}

// getPostgresURL extracts the PostgreSQL URL from a BaseApp instance
func getPostgresURL(app App) (string, error) {
	// Type assert to BaseApp to access the config
	baseApp, ok := app.(*BaseApp)
	if !ok {
		return "", errors.New("app is not a BaseApp instance")
	}
	return baseApp.config.PostgresURL, nil
}

// createPostgresDump creates a PostgreSQL dump using pg_dump
func createPostgresDump(connInfo *PostgresConnectionInfo, database, outputPath string) error {
	// Prepare pg_dump command
	args := []string{
		"pg_dump",
		"-h", connInfo.Host,
		"-p", connInfo.Port,
		"-U", connInfo.User,
		"-d", database,
		"--no-password",
		"--verbose",
		"--clean",
		"--if-exists",
		"--create",
		"--file", outputPath,
	}

	cmd := exec.Command(args[0], args[1:]...)

	// Set PGPASSWORD environment variable if password is provided
	if connInfo.Password != "" {
		cmd.Env = append(os.Environ(), "PGPASSWORD="+connInfo.Password)
	}

	// Capture output for debugging
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("pg_dump failed for database %s: %w\nStderr: %s", database, err, stderr.String())
	}

	return nil
}

// restorePostgresDumpSafe restores a PostgreSQL dump without dropping the database
func restorePostgresDumpSafe(connInfo *PostgresConnectionInfo, database, dumpPath string) error {
	// First, try to clean the database by dropping all tables/schemas
	cleanArgs := []string{
		"psql",
		"-h", connInfo.Host,
		"-p", connInfo.Port,
		"-U", connInfo.User,
		"-d", database,
		"--no-password",
		"-c", `
			DO $$
			DECLARE
				r RECORD;
			BEGIN
				-- Drop all tables
				FOR r IN (SELECT tablename FROM pg_tables WHERE schemaname = 'public') LOOP
					EXECUTE 'DROP TABLE IF EXISTS ' || quote_ident(r.tablename) || ' CASCADE';
				END LOOP;
				-- Drop all sequences
				FOR r IN (SELECT sequence_name FROM information_schema.sequences WHERE sequence_schema = 'public') LOOP
					EXECUTE 'DROP SEQUENCE IF EXISTS ' || quote_ident(r.sequence_name) || ' CASCADE';
				END LOOP;
				-- Drop all functions with proper signature handling
				FOR r IN (SELECT routine_name, specific_name FROM information_schema.routines WHERE routine_schema = 'public' AND routine_type = 'FUNCTION') LOOP
					BEGIN
						EXECUTE 'DROP FUNCTION IF EXISTS ' || quote_ident(r.specific_name) || ' CASCADE';
					EXCEPTION WHEN OTHERS THEN
						-- Ignore errors for functions that can't be dropped
						NULL;
					END;
				END LOOP;
				-- Drop all types
				FOR r IN (SELECT typname FROM pg_type WHERE typnamespace = (SELECT oid FROM pg_namespace WHERE nspname = 'public') AND typtype = 'c') LOOP
					EXECUTE 'DROP TYPE IF EXISTS ' || quote_ident(r.typname) || ' CASCADE';
				END LOOP;
			END $$;
		`,
	}

	cleanCmd := exec.Command(cleanArgs[0], cleanArgs[1:]...)

	// Set PGPASSWORD environment variable if password is provided
	if connInfo.Password != "" {
		cleanCmd.Env = append(os.Environ(), "PGPASSWORD="+connInfo.Password)
	}

	// Capture output for debugging
	var cleanStderr bytes.Buffer
	cleanCmd.Stderr = &cleanStderr

	if err := cleanCmd.Run(); err != nil {
		// Log warning but don't fail - database might be empty
		fmt.Printf("Warning: failed to clean database %s: %v\nStderr: %s\n", database, err, cleanStderr.String())
	}

	// Now restore from the dump file
	restoreArgs := []string{
		"psql",
		"-h", connInfo.Host,
		"-p", connInfo.Port,
		"-U", connInfo.User,
		"-d", database,
		"--no-password",
		"-f", dumpPath,
	}

	restoreCmd := exec.Command(restoreArgs[0], restoreArgs[1:]...)

	// Set PGPASSWORD environment variable if password is provided
	if connInfo.Password != "" {
		restoreCmd.Env = append(os.Environ(), "PGPASSWORD="+connInfo.Password)
	}

	// Capture output for debugging
	var restoreStderr bytes.Buffer
	restoreCmd.Stderr = &restoreStderr

	err := restoreCmd.Run()
	if err != nil {
		return fmt.Errorf("psql restore failed for database %s: %w\nStderr: %s", database, err, restoreStderr.String())
	}

	return nil
}

// restorePostgresDump restores a PostgreSQL dump using psql
func restorePostgresDump(connInfo *PostgresConnectionInfo, database, dumpPath string) error {
	// Try the safe method first (without dropping database)
	if err := restorePostgresDumpSafe(connInfo, database, dumpPath); err != nil {
		// If safe method fails, try the original method with database recreation
		fmt.Printf("Safe restore failed, trying with database recreation: %v\n", err)

		// First, try to drop and recreate the database
		if err := dropAndCreateDatabase(connInfo, database); err != nil {
			return fmt.Errorf("failed to recreate database %s: %w", database, err)
		}

		// Prepare psql command to restore the dump
		args := []string{
			"psql",
			"-h", connInfo.Host,
			"-p", connInfo.Port,
			"-U", connInfo.User,
			"-d", database,
			"--no-password",
			"-f", dumpPath,
		}

		cmd := exec.Command(args[0], args[1:]...)

		// Set PGPASSWORD environment variable if password is provided
		if connInfo.Password != "" {
			cmd.Env = append(os.Environ(), "PGPASSWORD="+connInfo.Password)
		}

		// Capture output for debugging
		var stderr bytes.Buffer
		cmd.Stderr = &stderr

		err := cmd.Run()
		if err != nil {
			return fmt.Errorf("psql restore failed for database %s: %w\nStderr: %s", database, err, stderr.String())
		}
	}

	return nil
}

// dropAndCreateDatabase drops and recreates a PostgreSQL database
func dropAndCreateDatabase(connInfo *PostgresConnectionInfo, database string) error {
	// Connect to the default 'postgres' database to perform admin operations
	adminConnInfo := *connInfo
	adminConnInfo.Database = "postgres"

	// First, terminate all active connections to the target database
	terminateArgs := []string{
		"psql",
		"-h", adminConnInfo.Host,
		"-p", adminConnInfo.Port,
		"-U", adminConnInfo.User,
		"-d", adminConnInfo.Database,
		"--no-password",
		"-c", fmt.Sprintf(`
			SELECT pg_terminate_backend(pid)
			FROM pg_stat_activity
			WHERE datname = '%s' AND pid <> pg_backend_pid();
		`, database),
	}

	terminateCmd := exec.Command(terminateArgs[0], terminateArgs[1:]...)
	if adminConnInfo.Password != "" {
		terminateCmd.Env = append(os.Environ(), "PGPASSWORD="+adminConnInfo.Password)
	}

	// Ignore errors from terminate command as the database might not exist or have no connections
	_ = terminateCmd.Run()

	// Drop database if exists
	dropArgs := []string{
		"psql",
		"-h", adminConnInfo.Host,
		"-p", adminConnInfo.Port,
		"-U", adminConnInfo.User,
		"-d", adminConnInfo.Database,
		"--no-password",
		"-c", fmt.Sprintf(`DROP DATABASE IF EXISTS "%s"`, database),
	}

	dropCmd := exec.Command(dropArgs[0], dropArgs[1:]...)
	if adminConnInfo.Password != "" {
		dropCmd.Env = append(os.Environ(), "PGPASSWORD="+adminConnInfo.Password)
	}

	// Capture stderr for better error reporting
	var dropStderr bytes.Buffer
	dropCmd.Stderr = &dropStderr

	if err := dropCmd.Run(); err != nil {
		return fmt.Errorf("failed to drop database %s: %w\nStderr: %s", database, err, dropStderr.String())
	}

	// Create database
	createArgs := []string{
		"psql",
		"-h", adminConnInfo.Host,
		"-p", adminConnInfo.Port,
		"-U", adminConnInfo.User,
		"-d", adminConnInfo.Database,
		"--no-password",
		"-c", fmt.Sprintf(`CREATE DATABASE "%s"`, database),
	}

	createCmd := exec.Command(createArgs[0], createArgs[1:]...)
	if adminConnInfo.Password != "" {
		createCmd.Env = append(os.Environ(), "PGPASSWORD="+adminConnInfo.Password)
	}

	// Capture stderr for better error reporting
	var createStderr bytes.Buffer
	createCmd.Stderr = &createStderr

	if err := createCmd.Run(); err != nil {
		return fmt.Errorf("failed to create database %s: %w\nStderr: %s", database, err, createStderr.String())
	}

	return nil
}

// CreateBackup creates a new backup of the current app pb_data directory and PostgreSQL databases.
//
// If name is empty, it will be autogenerated.
// If backup with the same name exists, the new backup file will replace it.
//
// The backup is executed within a transaction, meaning that new writes
// will be temporary "blocked" until the backup file is generated.
//
// To safely perform the backup, it is recommended to have free disk space
// for at least 2x the size of the pb_data directory plus database dump sizes.
//
// By default backups are stored in pb_data/backups
// (the backups directory itself is excluded from the generated backup).
//
// When using S3 storage for the uploaded collection files, you have to
// take care manually to backup those since they are not part of the pb_data.
//
// Backups can be stored on S3 if it is configured in app.Settings().Backups.
func (app *BaseApp) CreateBackup(ctx context.Context, name string) error {
	if app.Store().Has(StoreKeyActiveBackup) {
		return errors.New("try again later - another backup/restore operation has already been started")
	}

	app.Store().Set(StoreKeyActiveBackup, name)
	defer app.Store().Remove(StoreKeyActiveBackup)

	event := new(BackupEvent)
	event.App = app
	event.Context = ctx
	event.Name = name
	// default root dir entries to exclude from the backup generation
	event.Exclude = []string{LocalBackupsDirName, LocalTempDirName, LocalAutocertCacheDirName}

	return app.OnBackupCreate().Trigger(event, func(e *BackupEvent) error {
		// generate a default name if missing
		if e.Name == "" {
			e.Name = generateBackupName(e.App, "pb_backup_")
		}

		// make sure that the special temp directory exists
		// note: it needs to be inside the current pb_data to avoid "cross-device link" errors
		localTempDir := filepath.Join(e.App.DataDir(), LocalTempDirName)
		if err := os.MkdirAll(localTempDir, os.ModePerm); err != nil {
			return fmt.Errorf("failed to create a temp dir: %w", err)
		}

		// Parse PostgreSQL connection info
		postgresURL, err := getPostgresURL(e.App)
		if err != nil {
			return fmt.Errorf("failed to get PostgreSQL URL: %w", err)
		}

		connInfo, err := parsePostgresURL(postgresURL)
		if err != nil {
			return fmt.Errorf("failed to parse PostgreSQL connection URL: %w", err)
		}

		// Create temporary backup directory
		tempBackupDir := filepath.Join(localTempDir, "pb_backup_"+security.PseudorandomString(6))
		if err := os.MkdirAll(tempBackupDir, os.ModePerm); err != nil {
			return fmt.Errorf("failed to create temp backup dir: %w", err)
		}
		defer os.RemoveAll(tempBackupDir)

		// Create database dumps and copy files within a transaction
		createErr := e.App.RunInTransaction(func(txApp App) error {
			return txApp.AuxRunInTransaction(func(txApp App) error {
				// Create PostgreSQL dumps
				dataDbDumpPath := filepath.Join(tempBackupDir, "data.pg-dump.sql")
				auxDbDumpPath := filepath.Join(tempBackupDir, "auxiliary.pg-dump.sql")

				// Get database names using the proper App interface methods
				dataDbName := txApp.PostgresDataDB()
				auxDbName := txApp.PostgresAuxDB()

				app.Logger().Info("Creating PostgreSQL dump for data database", "database", dataDbName)
				if err := createPostgresDump(connInfo, dataDbName, dataDbDumpPath); err != nil {
					return fmt.Errorf("failed to create data database dump: %w", err)
				}

				app.Logger().Info("Creating PostgreSQL dump for auxiliary database", "database", auxDbName)
				if err := createPostgresDump(connInfo, auxDbName, auxDbDumpPath); err != nil {
					return fmt.Errorf("failed to create auxiliary database dump: %w", err)
				}

				// Copy storage directory if it exists
				storageDir := filepath.Join(txApp.DataDir(), LocalStorageDirName)
				if _, err := os.Stat(storageDir); err == nil {
					destStorageDir := filepath.Join(tempBackupDir, LocalStorageDirName)
					if err := copyDir(storageDir, destStorageDir); err != nil {
						return fmt.Errorf("failed to copy storage directory: %w", err)
					}
				}

				// Copy any other necessary files (exclude databases, backups, temp dirs)
				entries, err := os.ReadDir(txApp.DataDir())
				if err != nil {
					return fmt.Errorf("failed to read data directory: %w", err)
				}

				for _, entry := range entries {
					name := entry.Name()

					// Skip excluded directories and database files
					skip := false
					for _, exclude := range e.Exclude {
						if name == exclude {
							skip = true
							break
						}
					}

					// Skip database files and storage directory (already handled above)
					if skip || name == LocalStorageDirName ||
					   name == "data.db" || name == "auxiliary.db" ||
					   name == "data.db-wal" || name == "data.db-shm" ||
					   name == "auxiliary.db-wal" || name == "auxiliary.db-shm" {
						continue
					}

					srcPath := filepath.Join(txApp.DataDir(), name)
					destPath := filepath.Join(tempBackupDir, name)

					if entry.IsDir() {
						if err := copyDir(srcPath, destPath); err != nil {
							return fmt.Errorf("failed to copy directory %s: %w", name, err)
						}
					} else {
						if err := copyFile(srcPath, destPath); err != nil {
							return fmt.Errorf("failed to copy file %s: %w", name, err)
						}
					}
				}

				return nil
			})
		})
		if createErr != nil {
			return createErr
		}

		// Create zip archive from the temp backup directory
		tempPath := filepath.Join(localTempDir, "pb_backup_"+security.PseudorandomString(6)+".zip")
		if err := archive.Create(tempBackupDir, tempPath); err != nil {
			return fmt.Errorf("failed to create backup archive: %w", err)
		}
		defer os.Remove(tempPath)

		// persist the backup in the backups filesystem
		// ---
		fsys, err := e.App.NewBackupsFilesystem()
		if err != nil {
			return err
		}
		defer fsys.Close()

		fsys.SetContext(e.Context)

		file, err := filesystem.NewFileFromPath(tempPath)
		if err != nil {
			return err
		}
		file.OriginalName = e.Name
		file.Name = file.OriginalName

		if err := fsys.UploadFile(file, file.Name); err != nil {
			return err
		}

		return nil
	})
}

// RestoreBackup restores the backup with the specified name and restarts
// the current running application process.
//
// NB! This feature is experimental and currently is expected to work only on UNIX based systems.
//
// To safely perform the restore it is recommended to have free disk space
// for at least 2x the size of the restored pb_data backup.
//
// The performed steps are:
//
//  1. Download the backup with the specified name in a temp location
//     (this is in case of S3; otherwise it creates a temp copy of the zip)
//
//  2. Extract the backup in a temp directory inside the app "pb_data"
//     (eg. "pb_data/.pb_temp_to_delete/pb_restore").
//
//  3. Restore PostgreSQL databases from the extracted SQL dumps.
//
//  4. Move the current app "pb_data" content (excluding the local backups and the special temp dir)
//     under another temp sub dir that will be deleted on the next app start up
//     (eg. "pb_data/.pb_temp_to_delete/old_pb_data").
//     This is because on some environments it may not be allowed
//     to delete the currently open "pb_data" files.
//
//  5. Move the extracted dir content to the app "pb_data".
//
//  6. Restart the app (on successful app bootstrap it will also remove the old pb_data).
//
// If a failure occurs during the restore process the dir changes are reverted.
// If for whatever reason the revert is not possible, it panics.
//
// Note that if your pb_data has custom network mounts as subdirectories, then
// it is possible the restore to fail during the `os.Rename` operations
// (see https://github.com/pocketbase/pocketbase/issues/4647).
func (app *BaseApp) RestoreBackup(ctx context.Context, name string) error {
	if app.Store().Has(StoreKeyActiveBackup) {
		return errors.New("try again later - another backup/restore operation has already been started")
	}

	app.Store().Set(StoreKeyActiveBackup, name)
	defer app.Store().Remove(StoreKeyActiveBackup)

	event := new(BackupEvent)
	event.App = app
	event.Context = ctx
	event.Name = name
	// default root dir entries to exclude from the backup restore
	event.Exclude = []string{LocalBackupsDirName, LocalTempDirName, LocalAutocertCacheDirName}

	return app.OnBackupRestore().Trigger(event, func(e *BackupEvent) error {
		if runtime.GOOS == "windows" {
			return errors.New("restore is not supported on Windows")
		}

		// make sure that the special temp directory exists
		// note: it needs to be inside the current pb_data to avoid "cross-device link" errors
		localTempDir := filepath.Join(e.App.DataDir(), LocalTempDirName)
		if err := os.MkdirAll(localTempDir, os.ModePerm); err != nil {
			return fmt.Errorf("failed to create a temp dir: %w", err)
		}

		fsys, err := e.App.NewBackupsFilesystem()
		if err != nil {
			return err
		}
		defer fsys.Close()

		fsys.SetContext(e.Context)

		if ok, _ := fsys.Exists(name); !ok {
			return fmt.Errorf("missing or invalid backup file %q to restore", name)
		}

		extractedDataDir := filepath.Join(localTempDir, "pb_restore_"+security.PseudorandomString(8))
		defer os.RemoveAll(extractedDataDir)

		// extract the zip
		if e.App.Settings().Backups.S3.Enabled {
			br, err := fsys.GetReader(name)
			if err != nil {
				return err
			}
			defer br.Close()

			// create a temp zip file from the blob.Reader and try to extract it
			tempZip, err := os.CreateTemp(localTempDir, "pb_restore_zip")
			if err != nil {
				return err
			}
			defer os.Remove(tempZip.Name())
			defer tempZip.Close() // note: this technically shouldn't be necessary but it is here to workaround platforms discrepancies

			_, err = io.Copy(tempZip, br)
			if err != nil {
				return err
			}

			err = archive.Extract(tempZip.Name(), extractedDataDir)
			if err != nil {
				return err
			}

			// remove the temp zip file since we no longer need it
			// (this is in case the app restarts and the defer calls are not called)
			_ = tempZip.Close()
			err = os.Remove(tempZip.Name())
			if err != nil {
				e.App.Logger().Warn(
					"[RestoreBackup] Failed to remove the temp zip backup file",
					slog.String("file", tempZip.Name()),
					slog.String("error", err.Error()),
				)
			}
		} else {
			// manually construct the local path to avoid creating a copy of the zip file
			// since the blob reader currently doesn't implement ReaderAt
			zipPath := filepath.Join(e.App.DataDir(), LocalBackupsDirName, filepath.Base(name))

			err = archive.Extract(zipPath, extractedDataDir)
			if err != nil {
				return err
			}
		}

		// Parse PostgreSQL connection info
		postgresURL, err := getPostgresURL(e.App)
		if err != nil {
			return fmt.Errorf("failed to get PostgreSQL URL: %w", err)
		}

		connInfo, err := parsePostgresURL(postgresURL)
		if err != nil {
			return fmt.Errorf("failed to parse PostgreSQL connection URL: %w", err)
		}

		// Restore PostgreSQL databases from dumps
		dataDbDumpPath := filepath.Join(extractedDataDir, "data.pg-dump.sql")
		auxDbDumpPath := filepath.Join(extractedDataDir, "auxiliary.pg-dump.sql")

		// Check if the required dump files exist
		if _, err := os.Stat(dataDbDumpPath); err != nil {
			return fmt.Errorf("data.pg-dump.sql file is missing or invalid: %w", err)
		}
		if _, err := os.Stat(auxDbDumpPath); err != nil {
			return fmt.Errorf("auxiliary.pg-dump.sql file is missing or invalid: %w", err)
		}

		// Get database names
		dataDbName := e.App.PostgresDataDB()
		auxDbName := e.App.PostgresAuxDB()

		// Restore PostgreSQL databases from dumps
		e.App.Logger().Info("Restoring PostgreSQL dump for data database", "database", dataDbName)
		if err := restorePostgresDump(connInfo, dataDbName, dataDbDumpPath); err != nil {
			return fmt.Errorf("failed to restore data database: %w", err)
		}

		e.App.Logger().Info("Restoring PostgreSQL dump for auxiliary database", "database", auxDbName)
		if err := restorePostgresDump(connInfo, auxDbName, auxDbDumpPath); err != nil {
			return fmt.Errorf("failed to restore auxiliary database: %w", err)
		}

		oldTempDataDir := filepath.Join(localTempDir, "old_pb_data_"+security.PseudorandomString(8))

		// Ensure the temp directory exists before trying to move files
		if err := os.MkdirAll(oldTempDataDir, os.ModePerm); err != nil {
			return fmt.Errorf("failed to create temp directory for old data: %w", err)
		}

		replaceErr := e.App.RunInTransaction(func(txApp App) error {
			return txApp.AuxRunInTransaction(func(txApp App) error {
				// move the current pb_data content to a special temp location
				// that will hold the old data between dirs replace
				// (the temp dir will be automatically removed on the next app start)
				if err := osutils.MoveDirContent(txApp.DataDir(), oldTempDataDir, e.Exclude...); err != nil {
					return fmt.Errorf("failed to move the current pb_data content to a temp location: %w", err)
				}

				// move the extracted archive content to the app's pb_data (excluding SQL dumps)
				entries, err := os.ReadDir(extractedDataDir)
				if err != nil {
					return fmt.Errorf("failed to read extracted directory: %w", err)
				}

				for _, entry := range entries {
					name := entry.Name()

					// Skip SQL dump files as they've already been restored to PostgreSQL
					if name == "data.pg-dump.sql" || name == "auxiliary.pg-dump.sql" {
						continue
					}

					srcPath := filepath.Join(extractedDataDir, name)
					destPath := filepath.Join(txApp.DataDir(), name)

					if entry.IsDir() {
						if err := copyDir(srcPath, destPath); err != nil {
							return fmt.Errorf("failed to copy directory %s: %w", name, err)
						}
					} else {
						if err := copyFile(srcPath, destPath); err != nil {
							return fmt.Errorf("failed to copy file %s: %w", name, err)
						}
					}
				}

				return nil
			})
		})
		if replaceErr != nil {
			return replaceErr
		}

		revertDataDirChanges := func() error {
			return e.App.RunInTransaction(func(txApp App) error {
				return txApp.AuxRunInTransaction(func(txApp App) error {
					if err := osutils.MoveDirContent(txApp.DataDir(), extractedDataDir, e.Exclude...); err != nil {
						return fmt.Errorf("failed to revert the extracted dir change: %w", err)
					}

					if err := osutils.MoveDirContent(oldTempDataDir, txApp.DataDir(), e.Exclude...); err != nil {
						return fmt.Errorf("failed to revert old pb_data dir change: %w", err)
					}

					return nil
				})
			})
		}

		// restart the app
		if err := e.App.Restart(); err != nil {
			if revertErr := revertDataDirChanges(); revertErr != nil {
				panic(revertErr)
			}

			return fmt.Errorf("failed to restart the app process: %w", err)
		}

		return nil
	})
}

// registerAutobackupHooks registers the autobackup app serve hooks.
func (app *BaseApp) registerAutobackupHooks() {
	const jobId = "__pbAutoBackup__"

	loadJob := func() {
		rawSchedule := app.Settings().Backups.Cron
		if rawSchedule == "" {
			app.Cron().Remove(jobId)
			return
		}

		app.Cron().Add(jobId, rawSchedule, func() {
			// Only run backups on leader instances
			if !app.IsLeader() {
				return
			}

			const autoPrefix = "@auto_pb_backup_"

			name := generateBackupName(app, autoPrefix)

			if err := app.CreateBackup(context.Background(), name); err != nil {
				app.Logger().Error(
					"[Backup cron] Failed to create backup",
					slog.String("name", name),
					slog.String("error", err.Error()),
				)
			}

			maxKeep := app.Settings().Backups.CronMaxKeep

			if maxKeep == 0 {
				return // no explicit limit
			}

			fsys, err := app.NewBackupsFilesystem()
			if err != nil {
				app.Logger().Error(
					"[Backup cron] Failed to initialize the backup filesystem",
					slog.String("error", err.Error()),
				)
				return
			}
			defer fsys.Close()

			files, err := fsys.List(autoPrefix)
			if err != nil {
				app.Logger().Error(
					"[Backup cron] Failed to list autogenerated backups",
					slog.String("error", err.Error()),
				)
				return
			}

			if maxKeep >= len(files) {
				return // nothing to remove
			}

			// sort desc
			sort.Slice(files, func(i, j int) bool {
				return files[i].ModTime.After(files[j].ModTime)
			})

			// keep only the most recent n auto backup files
			toRemove := files[maxKeep:]

			for _, f := range toRemove {
				if err := fsys.Delete(f.Key); err != nil {
					app.Logger().Error(
						"[Backup cron] Failed to remove old autogenerated backup",
						slog.String("key", f.Key),
						slog.String("error", err.Error()),
					)
				}
			}
		})
	}

	app.OnBootstrap().BindFunc(func(e *BootstrapEvent) error {
		if err := e.Next(); err != nil {
			return err
		}

		loadJob()

		return nil
	})

	app.OnSettingsReload().BindFunc(func(e *SettingsReloadEvent) error {
		if err := e.Next(); err != nil {
			return err
		}

		loadJob()

		return nil
	})
}

func generateBackupName(app App, prefix string) string {
	appName := inflector.Snakecase(app.Settings().Meta.AppName)
	if len(appName) > 50 {
		appName = appName[:50]
	}

	return fmt.Sprintf(
		"%s%s_%s.zip",
		prefix,
		appName,
		time.Now().UTC().Format("20060102150405"),
	)
}
