package kafka

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/mail"
	"strings"
	"time"

	kafka "github.com/segmentio/kafka-go"
	"github.com/sim-pez/we-regret-to-persist/internal/core/entity"
	"github.com/sim-pez/we-regret-to-persist/internal/core/usecase"
)

// emailDate is a time.Time that can unmarshal RFC 5322 / RFC 2822 email date strings.
type emailDate struct{ time.Time }

// fallbackDateFormats are tried when net/mail.ParseDate fails.
var fallbackDateFormats = []string{
	time.RFC3339,
	time.RFC1123Z, // "Mon, 02 Jan 2006 15:04:05 -0700"
	time.RFC1123,  // "Mon, 02 Jan 2006 15:04:05 MST"
	"Mon, _2 Jan 2006 15:04:05 -0700",
	"Mon, _2 Jan 2006 15:04:05 MST",
	"02 Jan 2006 15:04:05 -0700",
	"_2 Jan 2006 15:04:05 -0700",
	"02 Jan 2006 15:04:05 MST",
	"_2 Jan 2006 15:04:05 MST",
}

func (d *emailDate) UnmarshalJSON(b []byte) error {
	s := string(b)
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		s = s[1 : len(s)-1]
	}

	// net/mail.ParseDate is a proper RFC 5322 parser and handles parenthetical
	// timezone comments (e.g. "(GMT+01:00)") natively.
	if t, err := mail.ParseDate(s); err == nil {
		d.Time = t
		return nil
	}

	// Fallback: strip any trailing parenthetical and try known layouts.
	stripped := s
	if before, _, found := strings.Cut(s, " ("); found && strings.HasSuffix(s, ")") {
		stripped = before
	}
	for _, layout := range fallbackDateFormats {
		if t, err := time.Parse(layout, stripped); err == nil {
			d.Time = t
			return nil
		}
	}
	return fmt.Errorf("unrecognised date format: %s", s)
}

// kafkaEmailEvent mirrors the JSON shape of incoming Kafka messages.
type kafkaEmailEvent struct {
	From    string    `json:"from"`
	Subject string    `json:"subject"`
	Date    emailDate `json:"date"`
	Text    string    `json:"text"`
}

// Consumer reads email events from a Kafka topic and processes them.
type Consumer struct {
	reader  *kafka.Reader
	usecase *usecase.ProcessEmail
	logger  *slog.Logger
}

// NewConsumer creates a Consumer with at-least-once delivery semantics (manual commit).
func NewConsumer(logger *slog.Logger, broker, topic, groupID string, uc *usecase.ProcessEmail) *Consumer {
	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:        []string{broker},
		Topic:          topic,
		GroupID:        groupID,
		MinBytes:       1,
		MaxBytes:       10e6, // 10 MB
		CommitInterval: 0,    // manual commit via FetchMessage + CommitMessage
		StartOffset:    kafka.FirstOffset,
	})
	return &Consumer{reader: reader, usecase: uc, logger: logger}
}

// Run blocks, consuming messages until ctx is cancelled.
func (c *Consumer) Run(ctx context.Context) error {
	for {
		msg, err := c.reader.FetchMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return nil // clean shutdown
			}
			c.logger.Error("fetch message", "err", err)
			continue
		}

		var event kafkaEmailEvent
		if err := json.Unmarshal(msg.Value, &event); err != nil {
			c.logger.Error("unmarshal message", "err", err, "offset", msg.Offset)
			return err
		}

		email := &entity.Email{
			From:    event.From,
			Subject: event.Subject,
			Date:    event.Date.Time,
			Text:    event.Text,
		}

		if err := c.usecase.Execute(ctx, email); err != nil {
			return fmt.Errorf("process email at offset %d: %w", msg.Offset, err)
		}

		if err := c.reader.CommitMessages(ctx, msg); err != nil {
			c.logger.Error("commit message", "err", err, "offset", msg.Offset)
		}

		c.logger.Info("email processed", "from", event.From, "subject", event.Subject, "offset", msg.Offset)
	}
}

// Close shuts down the Kafka reader, committing any pending offsets.
func (c *Consumer) Close() error {
	return c.reader.Close()
}
