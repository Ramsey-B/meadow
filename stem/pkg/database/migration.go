package database

import (
	"fmt"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Gobusters/ectologger"
	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/pkg/errors"
)

type MigrationLogger struct {
	ectologger.Logger
}

func (l MigrationLogger) Verbose() bool {
	return true
}

func (l MigrationLogger) Printf(format string, v ...any) {
	l.Infof(format, v...)
}

type Logger interface {

	// Printf is like fmt.Printf
	Printf(format string, v ...any)

	// Verbose should return true when verbose logging output is wanted
	Verbose() bool
}

type MigrationService struct {
	config *MigrationConfig
	logger ectologger.Logger
}

type MigrationConfig struct {
	MigrationFolderPath string
	Version             uint
	Force               int
	AutoRollback        bool // If enabled, will attempt to rollback the database to the previous version if an error occurs
}

func NewMigrationService(logger ectologger.Logger, config *MigrationConfig) *MigrationService {
	return &MigrationService{
		config: config,
		logger: logger,
	}
}

func (ms *MigrationService) resolveMigrationFolder() string {
	migrationFolder := ms.config.MigrationFolderPath
	if _, err := os.Stat(migrationFolder); err == nil {
		return migrationFolder
	}
	workingDirectory, _ := os.Getwd()
	separator := ""
	if workingDirectory != "/" {
		separator = "/"
	}
	migrationFolder = workingDirectory + separator + migrationFolder
	if _, err := os.Stat(migrationFolder); err == nil {
		return migrationFolder
	}
	return migrationFolder
}

func (ms *MigrationService) Migrate(databaseName string, databaseInstance database.Driver) error {
	migrationFolder := ms.resolveMigrationFolder()
	if _, err := os.Stat(migrationFolder); err != nil {
		return errors.Wrap(err, fmt.Sprintf("migration folder %s does not exist", migrationFolder))
	}

	m, err := migrate.NewWithDatabaseInstance("file://"+migrationFolder, databaseName, databaseInstance)
	if err != nil {
		ms.logger.WithError(err).Error("Failed to create migrate instance")
		return err
	}

	m.Log = MigrationLogger{Logger: ms.logger}

	return ms.runMigration(m)
}

func (ms *MigrationService) runMigration(m *migrate.Migrate) error {
	if ms.config.Force != 0 {
		// Force the database to a specific version
		err := m.Force(ms.config.Force)
		if err != nil {
			ms.logger.WithError(err).Errorf("Failed to force database to version %d", ms.config.Force)
			return err
		}
	}

	// get current version
	version, _, versionErr := m.Version()
	if versionErr != nil {
		ms.logger.WithError(versionErr).Error("Failed to get current migration version")
		version = 0
	}

	// Start logging progress
	done := make(chan bool)
	go ms.logProgress(done)

	startTime := time.Now()

	// migrate to the specified version or latest version
	var migrationErr error
	if ms.config.Version != 0 {
		migrationErr = m.Migrate(ms.config.Version)
	} else {
		migrationErr = m.Up()
	}

	// Stop logging progress
	done <- true

	elapsedTime := time.Since(startTime)
	ms.logger.Infof("Database migrations completed in %v", elapsedTime)

	return ms.handleMigrationError(m, migrationErr, version)
}

func (ms *MigrationService) logProgress(done chan bool) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	dots := 0
	for {
		select {
		case <-done:
			return
		case <-ticker.C:
			dots = (dots + 1) % 4
			ms.logger.Debugf("Executing database migrations%s", strings.Repeat(".", dots))
		}
	}
}

func (ms *MigrationService) handleMigrationError(m *migrate.Migrate, err error, previousVersion uint) error {
	// no error so we can return
	if err == nil {
		ms.logger.Info("Successfully applied migrations")
		return nil
	}

	// no new migrations to apply so we can return
	if err == migrate.ErrNoChange {
		ms.logger.Info("No new migrations to apply")
		return nil
	}

	// if the error is due to no migration found for version. This is usually due to a rollback.
	if strings.Contains(err.Error(), "no migration found for version") {
		// Get the latest available migration version from the folder
		latest, err := getLatestVersion(ms.resolveMigrationFolder())
		if err != nil {
			ms.logger.WithError(err).Error("Failed to get latest migration version")
		}
		ms.logger.Warnf("No migration found for version %d. Latest version is %d", previousVersion, latest)
		ms.logger.Infof("Forcing database to version %d", latest)
		// Force the database to the latest version available
		err = m.Force(latest)
		if err != nil {
			ms.logger.WithError(err).Errorf("Failed to force database to version %d", latest)
			return err
		}
		return nil
	}

	// Log the actual migration error first (before any rollback)
	ms.logger.WithError(err).Errorf("Migration failed with error: %v", err)

	// get current version
	version, dirty, versionErr := m.Version()
	if versionErr != nil && versionErr != migrate.ErrNilVersion {
		ms.logger.WithError(versionErr).Error("Failed to get current migration version")
	} else if ms.config.AutoRollback {
		// if we dont have a previous version, set it to
		if previousVersion == 0 {
			previousVersion = version - 1 // set it back to the previous version
		}

		// if the database is dirty, we want to revert to the previous version
		if dirty {
			// If the database is dirty, we want to set it clean and revert to the previous version
			ms.logger.Warnf("Database is dirty at version %d. Reverting to version %d", version, previousVersion)
			ms.logger.WithError(err).Errorf("Original migration error (before rollback): %v", err)

			// Revert to the previous version
			err = m.Force(int(previousVersion))
			if err != nil {
				ms.logger.WithError(err).Errorf("Failed to force database to version %d", previousVersion)
				return err
			}
		}

		// still return error even if we have reverted to the previous version to prevent the application from starting.
		return err
	}

	ms.logger.WithError(err).Errorf("Failed to apply migrations. Database version is dirty=%t at version %d", dirty, version)
	return err
}

func getLatestVersion(folderPath string) (int, error) {
	files, err := os.ReadDir(folderPath)
	if err != nil {
		return 0, err
	}

	var versions []int
	re := regexp.MustCompile(`^(\d+)_.*\.up\.sql$`)

	for _, file := range files {
		if !file.IsDir() {
			matches := re.FindStringSubmatch(file.Name())
			if len(matches) > 1 {
				version, err := strconv.Atoi(matches[1])
				if err != nil {
					return 0, err
				}
				versions = append(versions, version)
			}
		}
	}

	if len(versions) == 0 {
		return 0, fmt.Errorf("no migration files found")
	}

	sort.Ints(versions)
	return versions[len(versions)-1], nil
}

