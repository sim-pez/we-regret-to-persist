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
{"company":"<name or empty string if unrelated>","status":"applied|rejected|advanced|unrelated"}

Rules:
- status=rejected: the process is over for this role. Rejections are often politely worded — look for the underlying meaning, not keywords. The company is moving forward with other candidates, the position is filled, or they wish you luck in your search.
- status=applied: confirmation that an application was received or will be reviewed
- status=advanced: interview invite, offer, assessment, recruiter outreach about a role, or any next step
- status=unrelated: everything else, including networking emails, recruiter outreach that doesn't reference a specific role, and any ambiguous emails that don't clearly indicate the status of an application`

type claudeResponse struct {
	Company string `json:"company"`
	Status  string `json:"status"`
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

func (c *ClaudeClient) Execute(ctx context.Context, email *entity.Email) (string, entity.ApplicationStatus, error) {
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
		return "", "", fmt.Errorf("claude api call: %w", err)
	}
	if len(msg.Content) == 0 {
		return "", "", fmt.Errorf("claude returned empty response")
	}

	raw := "{" + msg.Content[0].Text
	var resp claudeResponse
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		c.logger.Warn("failed to unmarshal claude response, retrying", "error", err, "response", raw)

		msg, err = c.client.Messages.New(ctx, params)
		if err != nil {
			return "", "", fmt.Errorf("claude api call (retry): %w", err)
		}
		if len(msg.Content) == 0 {
			c.logger.Warn("claude returned empty response on retry")
			return "", "", fmt.Errorf("claude returned empty response on retry")
		}

		raw = "{" + msg.Content[0].Text
		if err := json.Unmarshal([]byte(raw), &resp); err != nil {
			c.logger.Warn("failed to unmarshal claude response after retry", "error", err, "response", raw)
			return "", "", fmt.Errorf("failed to unmarshal claude response after retry")
		}
	}

	status := entity.ApplicationStatus(resp.Status)
	switch status {
	case entity.ApplicationStatusApplied, entity.ApplicationStatusRejected,
		entity.ApplicationStatusAdvanced, entity.ApplicationStatusUnrelated:
		// ok
	default:
		return "", "", fmt.Errorf("unexpected status from claude: %q", status)
	}

	company := normalizeCompany(resp.Company)

	return company, status, nil
}

func normalizeCompany(s string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsPunct(r) {
			return -1
		}
		return unicode.ToLower(r)
	}, s)
}
