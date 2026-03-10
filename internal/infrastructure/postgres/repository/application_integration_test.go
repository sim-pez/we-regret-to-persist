//go:build integration

package repository_test

import (
	"context"
	"testing"
	"time"

	"fmt"

	"github.com/sim-pez/we-regret-to-persist/internal/core/entity"
	pg "github.com/sim-pez/we-regret-to-persist/internal/infrastructure/postgres"
	"github.com/sim-pez/we-regret-to-persist/internal/infrastructure/postgres/repository"
	"github.com/testcontainers/testcontainers-go"
	pgcontainer "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

func setupRepo(t *testing.T) *repository.PostgresqlRepository {
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

	return repository.NewPostgresqlRepository(db)
}

func ptr[T any](v T) *T { return &v }

func TestUpsertApplication_Insert(t *testing.T) {
	repo := setupRepo(t)
	ctx := context.Background()
	appliedAt := time.Date(2024, 1, 10, 0, 0, 0, 0, time.UTC)

	err := repo.UpsertApplication(ctx, "acme", func(existing *entity.Application) (bool, *entity.Application) {
		if existing != nil {
			t.Error("expected nil existing on first insert")
		}
		return true, &entity.Application{
			Company:   "acme",
			Status:    entity.ApplicationStatusApplied,
			AppliedAt: &appliedAt,
		}
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// verify by reading back via a second upsert call
	var got *entity.Application
	_ = repo.UpsertApplication(ctx, "acme", func(existing *entity.Application) (bool, *entity.Application) {
		got = existing
		return false, nil
	})
	if got == nil {
		t.Fatal("expected row to exist after insert")
	}
	if got.Status != entity.ApplicationStatusApplied {
		t.Errorf("status: got %q, want %q", got.Status, entity.ApplicationStatusApplied)
	}
	if got.AppliedAt == nil || !got.AppliedAt.Equal(appliedAt) {
		t.Errorf("applied_at: got %v, want %v", got.AppliedAt, appliedAt)
	}
}

func TestUpsertApplication_Update(t *testing.T) {
	repo := setupRepo(t)
	ctx := context.Background()
	appliedAt := time.Date(2024, 1, 10, 0, 0, 0, 0, time.UTC)
	rejectedAt := time.Date(2024, 2, 1, 0, 0, 0, 0, time.UTC)

	// first: insert applied
	_ = repo.UpsertApplication(ctx, "acme", func(_ *entity.Application) (bool, *entity.Application) {
		return true, &entity.Application{
			Company:   "acme",
			Status:    entity.ApplicationStatusApplied,
			AppliedAt: &appliedAt,
		}
	})

	// second: reject
	err := repo.UpsertApplication(ctx, "acme", func(existing *entity.Application) (bool, *entity.Application) {
		if existing == nil {
			t.Fatal("expected existing application on second call")
		}
		existing.Status = entity.ApplicationStatusRejected
		existing.RejectedAt = &rejectedAt
		return true, existing
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// verify
	var got *entity.Application
	_ = repo.UpsertApplication(ctx, "acme", func(existing *entity.Application) (bool, *entity.Application) {
		got = existing
		return false, nil
	})
	if got == nil {
		t.Fatal("expected row to exist")
	}
	if got.Status != entity.ApplicationStatusRejected {
		t.Errorf("status: got %q, want %q", got.Status, entity.ApplicationStatusRejected)
	}
	if got.RejectedAt == nil || !got.RejectedAt.Equal(rejectedAt) {
		t.Errorf("rejected_at: got %v, want %v", got.RejectedAt, rejectedAt)
	}
}

func TestUpsertApplication_SkipWhenProceedFalse(t *testing.T) {
	repo := setupRepo(t)
	ctx := context.Background()

	err := repo.UpsertApplication(ctx, "ghost", func(_ *entity.Application) (bool, *entity.Application) {
		return false, nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// verify no row was created
	var got *entity.Application
	_ = repo.UpsertApplication(ctx, "ghost", func(existing *entity.Application) (bool, *entity.Application) {
		got = existing
		return false, nil
	})
	if got != nil {
		t.Error("expected no row when proceed=false")
	}
}

func TestUpsertApplication_NoDuplicateRows(t *testing.T) {
	repo := setupRepo(t)
	ctx := context.Background()
	appliedAt := time.Date(2024, 1, 10, 0, 0, 0, 0, time.UTC)

	insert := func() {
		_ = repo.UpsertApplication(ctx, "acme", func(_ *entity.Application) (bool, *entity.Application) {
			return true, &entity.Application{
				Company:   "acme",
				Status:    entity.ApplicationStatusApplied,
				AppliedAt: &appliedAt,
			}
		})
	}

	insert()

	// second insert should fail (unique constraint) or be handled by updateFn seeing existing
	callCount := 0
	repo.UpsertApplication(ctx, "acme", func(existing *entity.Application) (bool, *entity.Application) { //nolint:errcheck
		callCount++
		if existing == nil {
			t.Error("expected existing row on second call, not nil")
		}
		return false, nil
	})
	if callCount != 1 {
		t.Errorf("updateFn called %d times, expected 1", callCount)
	}
}
