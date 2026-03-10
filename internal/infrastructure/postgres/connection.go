package postgres

import (
	"fmt"
	"time"

	"github.com/vinovest/sqlx"
)

func NewConnection(dsn string, maxOpenConns int, maxIdleConns int, connMaxLifetimeMins int) (*sqlx.DB, func() error, error) {
	dbConn, err := sqlx.Connect("postgres", dsn)
	if err != nil {
		return nil, nil, fmt.Errorf("cannot connect to postgres: %w", err)
	}

	// for a good explanation of those values, see https://www.alexedwards.net/blog/configuring-sqldb
	dbConn.SetMaxOpenConns(maxOpenConns)
	dbConn.SetMaxIdleConns(maxIdleConns)
	dbConn.SetConnMaxLifetime(time.Duration(connMaxLifetimeMins) * time.Minute)

	return dbConn, func() error { return dbConn.Close() }, nil
}
