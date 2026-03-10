package usecase

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/sim-pez/we-regret-to-persist/internal/core/entity"
)

type ProcessEmail interface {
	Execute(ctx context.Context, email *entity.Email) error
}

type Repository interface {
	UpsertApplication(ctx context.Context, company string, updateFn func(*entity.Application) (bool, *entity.Application)) error
}

type CompanyAndStatusExtractor interface {
	Execute(ctx context.Context, email *entity.Email) (string, entity.ApplicationStatus, bool)
}

type UpdateApplicationStatus struct {
	logger              *slog.Logger
	repo                Repository
	getCompanyAndStatus CompanyAndStatusExtractor
}

func NewUpdateApplicationStatus(logger *slog.Logger, repo Repository, getCompanyAndStatus CompanyAndStatusExtractor) *UpdateApplicationStatus {
	return &UpdateApplicationStatus{
		logger:              logger,
		repo:                repo,
		getCompanyAndStatus: getCompanyAndStatus,
	}
}

func (u *UpdateApplicationStatus) Execute(ctx context.Context, email *entity.Email) error {

	logger := u.logger.With("subject", email.Subject, "from", email.From)

	company, newStatus, proceed := u.getCompanyAndStatus.Execute(ctx, email)
	if !proceed {
		logger.Info("irrelevant email, skipping")
		return nil
	}

	updateFn := func(a *entity.Application) (bool, *entity.Application) {
		if a == nil && newStatus == entity.ApplicationStatusApplied { // new application
			logger.Info("new application", "company", company)
			return true, &entity.Application{
				Company:   company,
				AppliedAt: &email.Date,
				Status:    newStatus,
			}
		}
		if a == nil && newStatus == entity.ApplicationStatusRejected { // rejection email for previously unseen company
			return true, &entity.Application{
				Company:    company,
				RejectedAt: &email.Date,
				Status:     newStatus,
			}
		}
		if a == nil && newStatus == entity.ApplicationStatusAdvanced { // advanced status for previously unseen company
			logger.Info("advanced status for new company", "company", company)
			return true, &entity.Application{
				Company: company,
				Status:  newStatus,
			}
		}
		if newStatus == entity.ApplicationStatusRejected { // rejection email for existing application
			logger.Info("application rejected", "company", company)
			a.RejectedAt = &email.Date
			a.Status = newStatus
			return true, a
		}
		if newStatus == entity.ApplicationStatusAdvanced { // advanced status for existing application
			logger.Info("application advanced", "company", company)
			a.Status = newStatus
			return true, a
		}

		return false, nil
	}

	err := u.repo.UpsertApplication(ctx, company, updateFn)
	if err != nil {
		logger.Error("failed to upsert application", "company", company, "err", err)
		return fmt.Errorf("upsert application: %w", err)
	}

	logger.Info("application status updated", "company", company, "status", newStatus)

	return nil
}
