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

const systemPrompt = `Analyze emails for job application updates. Reply with JSON only, no other text:
{"company":"<name>","status":"applied|rejected|advanced","proceed":true|false}
proceed=false if not a job application email. status: applied=new application, rejected=rejection, advanced=interview/offer/next step.`

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

func (c *ClaudeClient) Execute(ctx context.Context, email *entity.Email) (string, entity.ApplicationStatus, bool) {
	userMsg := fmt.Sprintf("From: %s\nSubject: %s\n\n%s", email.From, email.Subject, email.Text)

	msg, err := c.client.Messages.New(ctx, anthropic.MessageNewParams{
		Model:     anthropic.ModelClaudeHaiku4_5_20251001,
		MaxTokens: 80,
		System: []anthropic.TextBlockParam{
			{Text: systemPrompt},
		},
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock(userMsg)),
			anthropic.NewAssistantMessage(anthropic.NewTextBlock("{")),
		},
	})
	if err != nil || len(msg.Content) == 0 {
		c.logger.Error("claude api call failed", "subject", email.Subject, "err", err)
		return "", "", false
	}

	var resp claudeResponse
	if err := json.Unmarshal([]byte("{"+msg.Content[0].Text), &resp); err != nil {
		c.logger.Error("failed to parse claude response", "subject", email.Subject, "err", err)
		return "", "", false
	}
	if !resp.Proceed {
		return "", "", false
	}

	status := entity.ApplicationStatus(resp.Status)
	if status != entity.ApplicationStatusApplied && status != entity.ApplicationStatusRejected && status != entity.ApplicationStatusAdvanced {
		c.logger.Error("claude returned invalid status", "subject", email.Subject, "status", resp.Status)
		return "", "", false
	}

	
	company := normalizeCompany(resp.Company)

	return company, status, true
}

func normalizeCompany(s string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsPunct(r) {
			return -1
		}
		return unicode.ToLower(r)
	}, s)
}
