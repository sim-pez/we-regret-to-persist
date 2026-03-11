package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"golang.org/x/sync/errgroup"

	"github.com/sim-pez/we-regret-to-persist/internal/core/usecase"
	"github.com/sim-pez/we-regret-to-persist/internal/infrastructure/client"
	"github.com/sim-pez/we-regret-to-persist/internal/infrastructure/config"
	"github.com/sim-pez/we-regret-to-persist/internal/infrastructure/kafka"
	"github.com/sim-pez/we-regret-to-persist/internal/infrastructure/logger"
	"github.com/sim-pez/we-regret-to-persist/internal/infrastructure/postgres"
	"github.com/sim-pez/we-regret-to-persist/internal/infrastructure/postgres/repository"
)

func main() {
	log := logger.New(slog.LevelInfo)

	cfg, err := config.LoadConfig()
	if err != nil {
		log.Error("load config", "err", err)
		os.Exit(1)
	}

	postgresDSN := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
		cfg.PostgresHost, cfg.PostgresPort, cfg.PostgresUser, cfg.PostgresPassword, cfg.PostgresDB)

	if err := postgres.ExecuteDBMigration(postgresDSN); err != nil && !errors.Is(err, postgres.ErrDBMigrationNoChange) {
		log.Error("db migration", "err", err)
		os.Exit(1)
	}

	db, closeDB, err := postgres.NewConnection(postgresDSN, 10, 5, 30)
	if err != nil {
		log.Error("db connection", "err", err)
		os.Exit(1)
	}
	defer func() {
		if err := closeDB(); err != nil {
			log.Error("close db", "err", err)
		}
	}()

	repo := repository.NewPostgresqlRepository(db)
	claudeClient := client.NewClaudeClient(log, cfg.ClaudeAPIKey)
	wordCount := usecase.NewWordCount(repo)
	uc := usecase.NewProcessEmail(log, repo, wordCount, claudeClient)

	consumer := kafka.NewConsumer(log, cfg.KafkaBroker, cfg.KafkaTopic, cfg.KafkaGroupID, uc)
	defer func() {
		if err := consumer.Close(); err != nil {
			log.Error("close consumer", "err", err)
		}
	}()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	g, ctx := errgroup.WithContext(ctx)
	g.Go(func() error {
		log.Info("starting consumer", "broker", cfg.KafkaBroker, "topic", cfg.KafkaTopic, "group", cfg.KafkaGroupID)
		return consumer.Run(ctx)
	})

	if err := g.Wait(); err != nil {
		log.Error("consumer error", "err", err)
		os.Exit(1)
	}
}
