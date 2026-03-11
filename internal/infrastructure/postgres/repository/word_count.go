package repository

import (
	"context"
	"fmt"

	"github.com/Masterminds/squirrel"
)

const (
	wordCountTable   = "word_count"
	columnWord       = "word"
	columnCount      = "count"
)

func (p *PostgresqlRepository) IncrementWordCounts(ctx context.Context, counts map[string]int) error {
	if len(counts) == 0 {
		return nil
	}

	q := squirrel.Insert(wordCountTable).
		Columns(columnWord, columnCount, columnUpdatedAt).
		Suffix("ON CONFLICT (word) DO UPDATE SET count = word_count.count + EXCLUDED.count, updated_at = NOW()").
		PlaceholderFormat(squirrel.Dollar)

	for word, delta := range counts {
		q = q.Values(word, delta, squirrel.Expr("NOW()"))
	}

	sqlStr, args, err := q.ToSql()
	if err != nil {
		return fmt.Errorf("cannot build sql query: %w", err)
	}

	_, err = p.db.ExecContext(ctx, sqlStr, args...)
	if err != nil {
		return fmt.Errorf("cannot upsert word counts: %w", err)
	}

	return nil
}
