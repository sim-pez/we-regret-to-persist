package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/Masterminds/squirrel"
	"github.com/sim-pez/we-regret-to-persist/internal/core/entity"
	"github.com/vinovest/sqlx"
)

const (
	applicationTable = "application"

	columnID         = "id"
	columnCompany    = "company"
	columnAppliedAt  = "applied_at"
	columnUpdatedAt  = "updated_at"
	columnRejectedAt = "rejected_at"
	columnStatus     = "status"
)

type applicationOnDB struct {
	ID         int        `db:"id"`
	Company    string     `db:"company"`
	AppliedAt  *time.Time `db:"applied_at"`
	UpdatedAt  time.Time  `db:"updated_at"`
	RejectedAt *time.Time `db:"rejected_at"`
	Status     string     `db:"status"`
}

func (a applicationOnDB) toEntity() entity.Application {
	return entity.Application{
		Company:    a.Company,
		AppliedAt:  a.AppliedAt,
		RejectedAt: a.RejectedAt,
		Status:     entity.ApplicationStatus(a.Status),
	}
}

func (p *PostgresqlRepository) UpsertApplication(ctx context.Context, company string, updateFn func(*entity.Application) (bool, *entity.Application)) error {
	tx, err := p.db.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("cannot start transaction: %w", err)
	}
	defer func() {
		err = finishTransaction(err, tx)
	}()

	applicationList, err := p.getApplication(tx, company)
	if err != nil {
		return fmt.Errorf("cannot get application: %w", err)
	}

	if len(applicationList) > 1 {
		return fmt.Errorf("multiple applications found for company: %s", company)
	}

	var existing *entity.Application
	if len(applicationList) == 1 {
		e := applicationList[0].toEntity()
		existing = &e
	}

	proceed, updated := updateFn(existing)
	if !proceed {
		return nil
	}

	if existing == nil {
		err = p.insertApplication(ctx, tx, company, updated)
	} else {
		err = p.updateApplication(ctx, tx, applicationList[0].ID, company, updated)
	}
	if err != nil {
		return fmt.Errorf("cannot upsert application: %w", err)
	}

	return nil
}

func (p *PostgresqlRepository) getApplication(tx *sqlx.Tx, company string) ([]applicationOnDB, error) {
	query := squirrel.Select("*").
		From(applicationTable).
		Where(squirrel.Eq{columnCompany: company}).
		PlaceholderFormat(squirrel.Dollar)

	sqlStr, args, err := query.ToSql()
	if err != nil {
		return nil, fmt.Errorf("cannot build sql query: %w", err)
	}

	var applications []applicationOnDB
	err = sqlx.Select(tx, &applications, sqlStr, args...)
	if err != nil && err != sql.ErrNoRows {
		return nil, fmt.Errorf("cannot select application: %w", err)
	}

	return applications, nil
}

func (p *PostgresqlRepository) insertApplication(ctx context.Context, tx *sqlx.Tx, _ string, a *entity.Application) error {
	query := squirrel.Insert(applicationTable).
		Columns(columnCompany, columnAppliedAt, columnRejectedAt, columnStatus).
		Values(a.Company, a.AppliedAt, a.RejectedAt, a.Status).
		PlaceholderFormat(squirrel.Dollar)

	sqlStr, args, err := query.ToSql()
	if err != nil {
		return fmt.Errorf("cannot build sql query: %w", err)
	}

	_, err = tx.ExecContext(ctx, sqlStr, args...)
	if err != nil {
		return fmt.Errorf("cannot insert application: %w", err)
	}

	return nil
}

func (p *PostgresqlRepository) updateApplication(ctx context.Context, tx *sqlx.Tx, id int, _ string, a *entity.Application) error {
	query := squirrel.Update(applicationTable).
		Set(columnCompany, a.Company).
		Set(columnAppliedAt, a.AppliedAt).
		Set(columnUpdatedAt, squirrel.Expr("NOW()")).
		Set(columnRejectedAt, a.RejectedAt).
		Set(columnStatus, a.Status).
		Where(squirrel.Eq{columnID: id}).
		PlaceholderFormat(squirrel.Dollar)

	sqlStr, args, err := query.ToSql()
	if err != nil {
		return fmt.Errorf("cannot build sql query: %w", err)
	}

	_, err = tx.ExecContext(ctx, sqlStr, args...)
	if err != nil {
		return fmt.Errorf("cannot update application: %w", err)
	}

	return nil
}
