package kafka

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	kafka "github.com/segmentio/kafka-go"
	"github.com/sim-pez/we-regret-to-persist/internal/core/entity"
	"github.com/sim-pez/we-regret-to-persist/internal/core/usecase"
)

// kafkaEmailEvent mirrors the JSON shape of incoming Kafka messages.
type kafkaEmailEvent struct {
	From    string    `json:"from"`
	Subject string    `json:"subject"`
	Date    time.Time `json:"date"`
	Text    string    `json:"text"`
}

// Consumer reads email events from a Kafka topic and processes them.
type Consumer struct {
	reader  *kafka.Reader
	usecase usecase.ProcessEmail
	logger  *slog.Logger
}

// NewConsumer creates a Consumer with at-least-once delivery semantics (manual commit).
func NewConsumer(logger *slog.Logger, broker, topic, groupID string, uc usecase.ProcessEmail) *Consumer {
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
			_ = c.reader.CommitMessages(ctx, msg) // skip poison pill
			continue
		}

		email := &entity.Email{
			From:    event.From,
			Subject: event.Subject,
			Date:    event.Date,
			Text:    event.Text,
		}

		if err := c.usecase.Execute(ctx, email); err != nil {
			// Transient error — do not commit, message will be retried.
			c.logger.Error("process email", "err", err, "offset", msg.Offset)
			select {
			case <-ctx.Done():
				return nil
			case <-time.After(5 * time.Second):
			}
			continue
		}

		if err := c.reader.CommitMessages(ctx, msg); err != nil {
			c.logger.Error("commit message", "err", err, "offset", msg.Offset)
		}

		c.logger.Info("email saved", "from", event.From, "subject", event.Subject, "offset", msg.Offset)
	}
}

// Close shuts down the Kafka reader, committing any pending offsets.
func (c *Consumer) Close() error {
	return c.reader.Close()
}
