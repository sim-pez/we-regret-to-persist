package usecase

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/sim-pez/we-regret-to-persist/internal/core/entity"
)

type ProcessEmailWithWordCount struct {
	logger              *slog.Logger
	repo                Repository
	wordCountRepo       WordCountRepository
	getCompanyAndStatus CompanyAndStatusExtractor
}

func NewProcessEmailWithWordCount(logger *slog.Logger, repo Repository, wordCountRepo WordCountRepository, getCompanyAndStatus CompanyAndStatusExtractor) *ProcessEmailWithWordCount {
	return &ProcessEmailWithWordCount{
		logger:              logger,
		repo:                repo,
		wordCountRepo:       wordCountRepo,
		getCompanyAndStatus: getCompanyAndStatus,
	}
}

func (p *ProcessEmailWithWordCount) Execute(ctx context.Context, email *entity.Email) error {
	logger := p.logger.With("subject", email.Subject, "from", email.From)

	company, newStatus, proceed := p.getCompanyAndStatus.Execute(ctx, email)
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

	if newStatus == entity.ApplicationStatusRejected {
		counts := countOccurrences(email.Text)
		if len(counts) > 0 {
			if err := p.wordCountRepo.IncrementWordCounts(ctx, counts); err != nil {
				logger.Error("increment word counts", "err", err)
				return err
			}
			logger.Info("word counts updated", "words_found", len(counts))
		}
	}

	return nil
}
