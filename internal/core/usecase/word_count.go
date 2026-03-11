package usecase

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/sim-pez/we-regret-to-persist/internal/core/entity"
)

var watchedWords = []string{
	// rejection classics
	"unfortunately", "regret", "unable", "cannot", "decline",
	"rejected", "unsuccessful", "reconsidering", "withdrawn",

	// corporate HR speak
	"opportunity",
	"experience", "forward", "culture", "fit", "aligned",
	"bandwidth", "capacity", "headcount", "pipeline", "requisition",
	"onboarding", "offboarding", "synergy", "leverage", "deliverable",
	"reorganization", "restructuring", "pivot", "transition", "change",
	"passionate", "cultural",

	// soft rejection phrases (word by word)
	"pursue", "candidates", "qualifications", "profile", "background",
	"consider", "competitive", "strong", "talent", "pool",
	"keep", "future", "openings", "role",

	// the hopeful ones
	"encourage", "again", "stay", "touch",
	"wish", "best", "success", "endeavors", "journey",
}

type WordCountRepository interface {
	IncrementWordCounts(ctx context.Context, counts map[string]int) error
}

type WordCount interface {
	Execute(ctx context.Context, logger *slog.Logger, email *entity.Email, status entity.ApplicationStatus) error
}

type wordCount struct {
	repo WordCountRepository
}

func NewWordCount(repo WordCountRepository) WordCount {
	return &wordCount{repo: repo}
}

func (w *wordCount) Execute(ctx context.Context, logger *slog.Logger, email *entity.Email, status entity.ApplicationStatus) error {
	if status != entity.ApplicationStatusRejected {
		return nil
	}

	counts := countOccurrences(email.Text)
	if len(counts) == 0 {
		return nil
	}

	if err := w.repo.IncrementWordCounts(ctx, counts); err != nil {
		return fmt.Errorf("increment word counts: %w", err)
	}

	logger.Info("word counts incremented", "words_found", len(counts))
	return nil
}

func countOccurrences(text string) map[string]int {
	watched := make(map[string]struct{}, len(watchedWords))
	for _, w := range watchedWords {
		watched[w] = struct{}{}
	}

	counts := make(map[string]int)
	for _, raw := range strings.Fields(text) {
		token := strings.ToLower(strings.Trim(raw, ".,!?;:\"'()[]{}"))
		if _, ok := watched[token]; ok {
			counts[token]++
		}
	}
	return counts
}
