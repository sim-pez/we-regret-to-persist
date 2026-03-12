package client

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"unicode"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/sim-pez/we-regret-to-persist/internal/core/entity"
)

const systemPrompt = `You are a job application tracker. Analyze emails and reply with JSON only, no other text:
{"company":"<name>","status":"applied|rejected|advanced","proceed":true|false}

Rules:
- proceed=false only if the email is clearly unrelated to a job application (newsletters, spam, etc.)
- status=rejected: any email indicating no further progress, including polite or euphemistic phrasing — "not moving forward", "move forward with other candidates", "decided to move forward with others", "not a fit", "went with other candidates", "wish you the best", "wish you all the best in your job search", "encourage you to apply in the future", "feel free to apply again", "keep your profile on file"
- status=applied: confirmation of a submitted application
- status=advanced: interview invite, offer, assessment, or any next step`

type claudeResponse struct {
	Company string `json:"company"`
	Status  string `json:"status"`
	Proceed bool   `json:"proceed"`
}

type ClaudeClient struct {
	client anthropic.Client
	logger *slog.Logger
}

func NewClaudeClient(logger *slog.Logger, apiKey string) *ClaudeClient {
	return &ClaudeClient{
		client: anthropic.NewClient(option.WithAPIKey(apiKey)),
		logger: logger,
	}
}

func (c *ClaudeClient) Execute(ctx context.Context, email *entity.Email) (string, entity.ApplicationStatus, bool, error) {
	userMsg := fmt.Sprintf("From: %s\nSubject: %s\n\n%s", email.From, email.Subject, email.Text)

	params := anthropic.MessageNewParams{
		Model:       anthropic.ModelClaudeHaiku4_5_20251001,
		MaxTokens:   80,
		Temperature: anthropic.Float(0),
		System: []anthropic.TextBlockParam{
			{Text: systemPrompt},
		},
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock(userMsg)),
			anthropic.NewAssistantMessage(anthropic.NewTextBlock("{")),
		},
	}

	msg, err := c.client.Messages.New(ctx, params)
	if err != nil {
		return "", "", false, fmt.Errorf("claude api call: %w", err)
	}
	if len(msg.Content) == 0 {
		return "", "", false, fmt.Errorf("claude returned empty response")
	}

	raw := "{" + msg.Content[0].Text
	var resp claudeResponse
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		c.logger.Warn("failed to unmarshal claude response, retrying", "error", err, "response", raw)

		msg, err = c.client.Messages.New(ctx, params)
		if err != nil {
			return "", "", false, fmt.Errorf("claude api call (retry): %w", err)
		}
		if len(msg.Content) == 0 {
			c.logger.Warn("claude returned empty response on retry")
			return "", "", false, nil
		}

		raw = "{" + msg.Content[0].Text
		if err := json.Unmarshal([]byte(raw), &resp); err != nil {
			c.logger.Warn("failed to unmarshal claude response after retry", "error", err, "response", raw)
			return "", "", false, nil
		}
	}

	if !resp.Proceed {
		return "", "", false, nil
	}

	status := entity.ApplicationStatus(resp.Status)
	if status != entity.ApplicationStatusApplied && status != entity.ApplicationStatusRejected && status != entity.ApplicationStatusAdvanced {
		return "", "", false, fmt.Errorf("claude returned invalid status: %q", resp.Status)
	}

	company := normalizeCompany(resp.Company)

	return company, status, true, nil
}

func normalizeCompany(s string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsPunct(r) {
			return -1
		}
		return unicode.ToLower(r)
	}, s)
}
