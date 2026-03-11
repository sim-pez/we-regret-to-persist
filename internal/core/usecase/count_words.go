package usecase

import (
	"context"
	"strings"
)

var watchedWords = []string{
	// rejection classics
	"unfortunately", "regret", "unable", "cannot", "decline",
	"rejected", "unsuccessful", "reconsidering", "withdrawn",

	// corporate HR speak
	"position", "interview", "application", "opportunity",
	"experience", "forward", "culture", "fit", "aligned",
	"bandwidth", "capacity", "headcount", "pipeline", "requisition",
	"onboarding", "offboarding", "synergy", "leverage", "deliverable",
	"reorganization", "restructuring", "pivot", "transition", "change",
	"passionate", "cultural",

	// soft rejection phrases (word by word)
	"pursue", "candidates", "qualifications", "profile", "background",
	"consider", "competitive", "strong", "talent", "pool",
	"keep", "file", "future", "openings", "role",

	// the hopeful ones
	"encourage", "apply", "again", "stay", "touch",
	"wish", "best", "success", "endeavors", "journey",
}

type WordCountRepository interface {
	IncrementWordCounts(ctx context.Context, counts map[string]int) error
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
