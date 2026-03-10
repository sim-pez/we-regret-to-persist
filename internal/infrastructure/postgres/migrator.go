package postgres

import (
	"errors"

	"github.com/golang-migrate/migrate/v4"
	migpg "github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"
)

// ErrDBMigrationNoChange is the error returned when no migration is needed
var ErrDBMigrationNoChange = errors.New("no change needed on db")

// ExecuteDBMigration runs migrate to reach up-to-date schema version
func ExecuteDBMigration(dsn string) error {
	dbConn, _, err := NewConnection(dsn, 1, 1, 5)
	if err != nil {
		return err
	}

	driver, err := migpg.WithInstance(dbConn.DB, &migpg.Config{})
	if err != nil {
		return err
	}

	source, err := iofs.New(Migrations, "migrations")
	if err != nil {
		return err
	}

	mig, err := migrate.NewWithInstance("iofs", source, "postgres", driver)
	if err != nil {
		return err
	}
	defer func() { _, _ = mig.Close() }()

	if err := mig.Up(); err != nil {
		if errors.Is(err, migrate.ErrNoChange) {
			return ErrDBMigrationNoChange
		}
		return err
	}

	return nil
}
