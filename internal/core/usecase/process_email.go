package usecase

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/sim-pez/we-regret-to-persist/internal/core/entity"
)

type ProcessEmail struct {
	logger              *slog.Logger
	repo                Repository
	wordCount           WordCount
	getCompanyAndStatus CompanyAndStatusExtractor
}

func NewProcessEmail(logger *slog.Logger, repo Repository, wordCount WordCount, getCompanyAndStatus CompanyAndStatusExtractor) *ProcessEmail {
	return &ProcessEmail{
		logger:              logger,
		repo:                repo,
		wordCount:           wordCount,
		getCompanyAndStatus: getCompanyAndStatus,
	}
}

func (p *ProcessEmail) Execute(ctx context.Context, email *entity.Email) error {
	logger := p.logger.With("subject", email.Subject, "from", email.From)

	company, newStatus, proceed, err := p.getCompanyAndStatus.Execute(ctx, email)
	if err != nil {
		return fmt.Errorf("extract company and status: %w", err)
	}
	classification := "other"
	if newStatus == entity.ApplicationStatusRejected {
		classification = "rejection"
	}
	if err := p.repo.InsertMailClassification(ctx, classification, email.Date); err != nil {
		logger.Error("failed to insert mail classification", "company", company, "err", err)
		return fmt.Errorf("insert mail classification: %w", err)
	}
	if !proceed {
		logger.Info("irrelevant email, skipping")
		return nil
	}

	logger.Info("classified as job email", "company", company, "status", newStatus)

	if err := p.repo.UpsertApplication(ctx, company, buildApplicationUpdateFn(logger, company, email, newStatus)); err != nil {
		logger.Error("failed to upsert application", "company", company, "err", err)
		return fmt.Errorf("upsert application: %w", err)
	}

	logger.Info("application status updated", "company", company, "status", newStatus)

	if err := p.wordCount.Execute(ctx, logger, email, newStatus); err != nil {
		logger.Error("failed to update word counts", "company", company, "err", err)
		return fmt.Errorf("update word counts: %w", err)
	}

	logger.Info("word counts updated", "company", company)

	return nil
}

func buildApplicationUpdateFn(logger *slog.Logger, company string, email *entity.Email, newStatus entity.ApplicationStatus) func(*entity.Application) (bool, *entity.Application) {
	return func(a *entity.Application) (bool, *entity.Application) {
		if a == nil && newStatus == entity.ApplicationStatusApplied {
			logger.Info("new application", "company", company)
			return true, &entity.Application{
				Company:   company,
				AppliedAt: &email.Date,
				Status:    newStatus,
			}
		}
		if a == nil && newStatus == entity.ApplicationStatusRejected {
			logger.Info("rejection for new company", "company", company)
			return true, &entity.Application{
				Company:    company,
				RejectedAt: &email.Date,
				Status:     newStatus,
			}
		}
		if a == nil && newStatus == entity.ApplicationStatusAdvanced {
			logger.Info("advanced status for new company", "company", company)
			return true, &entity.Application{
				Company: company,
				Status:  newStatus,
			}
		}
		if newStatus == entity.ApplicationStatusRejected {
			logger.Info("application rejected", "company", company)
			a.RejectedAt = &email.Date
			a.Status = newStatus
			return true, a
		}
		if newStatus == entity.ApplicationStatusAdvanced {
			logger.Info("application advanced", "company", company)
			a.Status = newStatus
			return true, a
		}
		return false, nil
	}
}
