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
