package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/Masterminds/squirrel"
)

const (
	historyTable         = "history"
	columnClassification = "classification"
	columnReceivedAt     = "received_at"
	maxHistoryRows       = 20
)

// InsertMailClassification inserts a new mail classification into the history table.
// To keep a maximum of 20 entries, the oldest rows beyond the limit are deleted before inserting.
func (p *PostgresqlRepository) InsertMailClassification(ctx context.Context, classification string, receivedAt time.Time) error {
	// Delete oldest rows if we are at or above the limit, keeping (maxHistoryRows - 1) so there
	// is room for the new entry without exceeding the cap.
	deleteSQL := fmt.Sprintf(`
		DELETE FROM %s
		WHERE id IN (
			SELECT id FROM %s
			ORDER BY received_at ASC
			LIMIT GREATEST(0, (SELECT COUNT(*) FROM %s) - %d)
		)`,
		historyTable, historyTable, historyTable, maxHistoryRows-1,
	)

	if _, err := p.db.ExecContext(ctx, deleteSQL); err != nil {
		return fmt.Errorf("cannot prune history: %w", err)
	}

	sqlStr, args, err := squirrel.Insert(historyTable).
		Columns(columnClassification, columnReceivedAt).
		Values(classification, receivedAt).
		PlaceholderFormat(squirrel.Dollar).
		ToSql()
	if err != nil {
		return fmt.Errorf("cannot build sql query: %w", err)
	}

	if _, err = p.db.ExecContext(ctx, sqlStr, args...); err != nil {
		return fmt.Errorf("cannot insert mail classification: %w", err)
	}

	return nil
}
