package repository

import (
	"github.com/vinovest/sqlx"
)

const columnUpdatedAt = "updated_at"

type PostgresqlRepository struct {
	db *sqlx.DB
}

func NewPostgresqlRepository(db *sqlx.DB) (*PostgresqlRepository) {
	return &PostgresqlRepository{
		db: db,
	}
}
