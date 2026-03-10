package repository

import (
	"fmt"

	"github.com/vinovest/sqlx"

)

type PostgresqlRepository struct {
	db *sqlx.DB
}

func NewPostgresqlRepository(db *sqlx.DB) (*PostgresqlRepository) {
	return &PostgresqlRepository{
		db: db,
	}
}


// finishTransaction rollbacks transaction if error is not nil, otherwise commits it
func finishTransaction(err error, tx *sqlx.Tx) error {
	if err != nil {
		if rollbackErr := tx.Rollback(); rollbackErr != nil {
			return fmt.Errorf("cannot rollback transaction: %w; original error: %v", rollbackErr, err)
		}
		return err
	}

	if commitErr := tx.Commit(); commitErr != nil {
		return fmt.Errorf("cannot commit transaction: %w", commitErr)
	}

	return nil
}