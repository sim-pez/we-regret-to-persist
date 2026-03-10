package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/sim-pez/we-regret-to-persist/internal/infrastructure/config"
	"github.com/sim-pez/we-regret-to-persist/internal/core/usecase"
	"github.com/sim-pez/we-regret-to-persist/internal/infrastructure/client"
	"github.com/sim-pez/we-regret-to-persist/internal/infrastructure/kafka"
	"github.com/sim-pez/we-regret-to-persist/internal/infrastructure/postgres"
	"github.com/sim-pez/we-regret-to-persist/internal/infrastructure/postgres/repository"
	"github.com/sim-pez/we-regret-to-persist/internal/infrastructure/logger"
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
	uc := usecase.NewUpdateApplicationStatus(log, repo, claudeClient)

	consumer := kafka.NewConsumer(log, cfg.KafkaBroker, cfg.KafkaTopic, cfg.KafkaGroupID, uc)
	defer func() {
		if err := consumer.Close(); err != nil {
			log.Error("close consumer", "err", err)
		}
	}()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	log.Info("starting consumer", "broker", cfg.KafkaBroker, "topic", cfg.KafkaTopic, "group", cfg.KafkaGroupID)
	if err := consumer.Run(ctx); err != nil {
		log.Error("consumer", "err", err)
		os.Exit(1)
	}
}
