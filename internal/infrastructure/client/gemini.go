package client

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"unicode"

	"google.golang.org/genai"

	"github.com/sim-pez/we-regret-to-persist/internal/core/entity"
)

const systemPrompt = `You are a job application tracker. Analyze emails and reply with JSON only, no other text:
{"company":"<name or empty string if unrelated>","status":"applied|rejected|advanced|unrelated"}

Rules:
- status=rejected: the process is over for this role. Rejections are often politely worded — look for the underlying meaning, not keywords. The company is moving forward with other candidates, the position is filled, or they wish you luck in your search.
- status=applied: confirmation that an application was received or will be reviewed
- status=advanced: interview invite, offer, assessment, recruiter outreach about a role, or any next step
- status=unrelated: everything else, including networking emails, recruiter outreach that doesn't reference a specific role, and any ambiguous emails that don't clearly indicate the status of an application`

var responseSchema = &genai.Schema{
	Type: genai.TypeObject,
	Properties: map[string]*genai.Schema{
		"company": {Type: genai.TypeString},
		"status": {
			Type: genai.TypeString,
			Enum: []string{
				string(entity.ApplicationStatusApplied),
				string(entity.ApplicationStatusRejected),
				string(entity.ApplicationStatusAdvanced),
				string(entity.ApplicationStatusUnrelated),
			},
		},
	},
	Required: []string{"company", "status"},
}

const geminiModel = "gemini-3.1-flash-lite"

type geminiResponse struct {
	Company string `json:"company"`
	Status  string `json:"status"`
}

type GeminiClient struct {
	client *genai.Client
	logger *slog.Logger
}

func NewGeminiClient(ctx context.Context, logger *slog.Logger, apiKey string) (*GeminiClient, error) {
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:  apiKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		return nil, fmt.Errorf("create gemini client: %w", err)
	}

	return &GeminiClient{
		client: client,
		logger: logger,
	}, nil
}

func (c *GeminiClient) Execute(ctx context.Context, email *entity.Email) (string, entity.ApplicationStatus, error) {
	userMsg := fmt.Sprintf("From: %s\nSubject: %s\n\n%s", email.From, email.Subject, email.Text)

	config := &genai.GenerateContentConfig{
		SystemInstruction: genai.NewContentFromText(systemPrompt, genai.RoleUser),
		Temperature:       genai.Ptr[float32](0),
		MaxOutputTokens:   256,
		ResponseMIMEType:  "application/json",
		ResponseSchema:    responseSchema,
	}

	resp, err := c.client.Models.GenerateContent(ctx, geminiModel, genai.Text(userMsg), config)
	if err != nil {
		return "", "", fmt.Errorf("gemini api call: %w", err)
	}

	raw := resp.Text()
	var parsed geminiResponse
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		c.logger.Warn("failed to unmarshal gemini response, retrying", "error", err, "response", raw)

		resp, err = c.client.Models.GenerateContent(ctx, geminiModel, genai.Text(userMsg), config)
		if err != nil {
			return "", "", fmt.Errorf("gemini api call (retry): %w", err)
		}

		raw = resp.Text()
		if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
			c.logger.Warn("failed to unmarshal gemini response after retry", "error", err, "response", raw)
			return "", "", fmt.Errorf("failed to unmarshal gemini response after retry")
		}
	}

	status := entity.ApplicationStatus(parsed.Status)
	switch status {
	case entity.ApplicationStatusApplied, entity.ApplicationStatusRejected,
		entity.ApplicationStatusAdvanced, entity.ApplicationStatusUnrelated:
		// ok
	default:
		return "", "", fmt.Errorf("unexpected status from gemini: %q", status)
	}

	company := normalizeCompany(parsed.Company)

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
