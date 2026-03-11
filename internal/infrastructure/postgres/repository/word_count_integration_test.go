//go:build integration

package repository_test

import (
	"context"
	"fmt"
	"testing"

	pg "github.com/sim-pez/we-regret-to-persist/internal/infrastructure/postgres"
	"github.com/sim-pez/we-regret-to-persist/internal/infrastructure/postgres/repository"
	"github.com/testcontainers/testcontainers-go"
	pgcontainer "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
	"github.com/vinovest/sqlx"
)

func setupWordCountRepo(t *testing.T) (*repository.PostgresqlRepository, *sqlx.DB) {
	t.Helper()
	ctx := context.Background()

	container, err := pgcontainer.Run(ctx,
		"postgres:16",
		pgcontainer.WithDatabase("testdb"),
		pgcontainer.WithUsername("test"),
		pgcontainer.WithPassword("test"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2),
		),
	)
	if err != nil {
		t.Fatalf("start postgres container: %v", err)
	}
	t.Cleanup(func() { _ = container.Terminate(ctx) })

	host, err := container.Host(ctx)
	if err != nil {
		t.Fatalf("get container host: %v", err)
	}
	mappedPort, err := container.MappedPort(ctx, "5432")
	if err != nil {
		t.Fatalf("get container port: %v", err)
	}

	dsn := fmt.Sprintf("host=%s port=%d user=test password=test dbname=testdb sslmode=disable", host, mappedPort.Int())

	if err := pg.ExecuteDBMigration(dsn); err != nil {
		t.Fatalf("run migrations: %v", err)
	}

	db, _, err := pg.NewConnection(dsn, 5, 5, 5)
	if err != nil {
		t.Fatalf("connect to db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	return repository.NewPostgresqlRepository(db), db
}

func getWordCount(t *testing.T, db *sqlx.DB, word string) (int64, bool) {
	t.Helper()
	var count int64
	err := db.QueryRowx("SELECT count FROM word_count WHERE word = $1", word).Scan(&count)
	if err != nil {
		return 0, false
	}
	return count, true
}

func TestIncrementWordCounts_Insert(t *testing.T) {
	repo, db := setupWordCountRepo(t)
	ctx := context.Background()

	err := repo.IncrementWordCounts(ctx, map[string]int{"unfortunately": 3, "regret": 1})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if count, ok := getWordCount(t, db, "unfortunately"); !ok || count != 3 {
		t.Errorf("unfortunately: got (%d, %v), want (3, true)", count, ok)
	}
	if count, ok := getWordCount(t, db, "regret"); !ok || count != 1 {
		t.Errorf("regret: got (%d, %v), want (1, true)", count, ok)
	}
}

func TestIncrementWordCounts_Accumulates(t *testing.T) {
	repo, db := setupWordCountRepo(t)
	ctx := context.Background()

	_ = repo.IncrementWordCounts(ctx, map[string]int{"unfortunately": 2})
	err := repo.IncrementWordCounts(ctx, map[string]int{"unfortunately": 5})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if count, ok := getWordCount(t, db, "unfortunately"); !ok || count != 7 {
		t.Errorf("unfortunately: got (%d, %v), want (7, true)", count, ok)
	}
}

func TestIncrementWordCounts_EmptyMapIsNoop(t *testing.T) {
	repo, db := setupWordCountRepo(t)
	ctx := context.Background()

	err := repo.IncrementWordCounts(ctx, map[string]int{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, ok := getWordCount(t, db, "unfortunately"); ok {
		t.Error("expected no rows after empty increment")
	}
}
