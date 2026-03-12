package usecase

import (
	"context"
	"time"

	"github.com/sim-pez/we-regret-to-persist/internal/core/entity"
)

type Repository interface {
	UpsertApplication(ctx context.Context, company string, updateFn func(*entity.Application) (bool, *entity.Application)) error
	InsertMailClassification(ctx context.Context, classification string, receivedAt time.Time) error
}

type CompanyAndStatusExtractor interface {
	Execute(ctx context.Context, email *entity.Email) (string, entity.ApplicationStatus, bool, error)
}
