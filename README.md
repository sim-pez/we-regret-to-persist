# we-regret-to-persist

[![CI](https://github.com/sim-pez/we-regret-to-persist/actions/workflows/ci.yml/badge.svg)](https://github.com/sim-pez/we-regret-to-persist/actions/workflows/ci.yml)
[![Go](https://img.shields.io/badge/go-1.24-blue?logo=go)](https://go.dev)
[![Claude](https://img.shields.io/badge/Claude-Haiku-blueviolet?logo=anthropic)](https://www.anthropic.com)
[![Docker](https://img.shields.io/badge/GHCR-ghcr.io%2Fsim--pez%2Fwe--regret--to--persist-blue?logo=docker)](https://github.com/sim-pez/we-regret-to-persist/pkgs/container/we-regret-to-persist)

An AI-powered job application tracker that automatically processes emails and maintains a database of your application statuses — so you never lose track of the rejections you'd rather forget.

## Overview

This service consumes job-related emails from a Kafka topic, uses Claude AI to classify each one, and upserts the result into PostgreSQL. It demonstrates a clean event-driven architecture with a focused use case: turning an inbox full of recruiter noise into a structured, queryable dataset.

```
Email → Kafka (Redpanda) → Claude AI → PostgreSQL
```

## Architecture

```
cmd/
└── status/              # Entry point: wires dependencies and starts the consumer

internal/
├── core/
│   ├── entity/          # Domain types: Email, Application, ApplicationStatus
│   └── usecase/         # Business logic: UpdateApplicationStatus
└── infrastructure/
    ├── client/          # Anthropic SDK integration (claude.go)
    ├── config/          # Environment-based configuration
    ├── kafka/           # Kafka consumer (segmentio/kafka-go)
    ├── logger/          # Structured logging (slog)
    └── postgres/        # DB connection, migrations, sqlx repository
```

The core layer has zero infrastructure dependencies. The use case accepts interfaces, making it straightforward to test in isolation.

## Key design decisions

- **Prefill prompting** — the Claude request prefills the assistant turn with `{` to force valid JSON output, avoiding any parsing preamble
- **Upsert semantics** — applications are keyed by `(email_from, company)`, so duplicate or updated emails converge to the latest status rather than creating duplicates
- **Embedded migrations** — SQL migrations are embedded in the binary via `go:embed`, so the service is self-contained and migrates on startup
- **Testcontainers** — integration tests spin up real Postgres and Kafka containers, no mocks

## Tech stack

| Layer | Technology |
|---|---|
| Language | Go 1.24 |
| AI | Claude Haiku (`anthropic-sdk-go`) |
| Messaging | Redpanda / Kafka (`segmentio/kafka-go`) |
| Database | PostgreSQL (`sqlx`, `squirrel`, `golang-migrate`) |
| Testing | `testify`, `testcontainers-go` |
| CI/CD | GitHub Actions → GHCR |

## Prerequisites

- Go 1.24+
- Docker & Docker Compose
- An [Anthropic API key](https://console.anthropic.com/)
- [`task`](https://taskfile.dev) (optional, but recommended)

## Getting started

**1. Configure environment**

```bash
cp .env.example .env
# Set CLAUDE_API_KEY and Postgres credentials
```

**2. Start infrastructure and seed sample emails**

```bash
task dev
```

This starts Redpanda and PostgreSQL, creates the Kafka topic, and produces the emails from `seed.jsonl`.

**3. Run the service**

```bash
task run
```

The service migrates the database on startup, then begins consuming from the `emails` topic.

<details>
<summary>Manual setup (without task)</summary>

```bash
docker compose up -d
go run ./cmd/status
```

</details>

## Configuration

All config is loaded from `.env` (see [.env.example](.env.example)).

| Variable | Default | Description |
|---|---|---|
| `CLAUDE_API_KEY` | — | Anthropic API key (required) |
| `POSTGRES_HOST` | `localhost` | |
| `POSTGRES_PORT` | `5432` | |
| `POSTGRES_USER` | — | |
| `POSTGRES_PASSWORD` | — | |
| `POSTGRES_DB` | — | |
| `KAFKA_BROKER` | `localhost:9092` | |
| `KAFKA_TOPIC` | `emails` | Topic to consume from |
| `KAFKA_GROUP_ID` | `we-regret-to-persist` | Consumer group |

## Kafka message format

Publish emails to the configured topic as JSON:

```json
{
  "from": "recruiter@company.com",
  "subject": "Your application at Acme Corp",
  "date": "2026-03-11T10:00:00Z",
  "text": "Thank you for applying..."
}
```

## Application statuses

Claude classifies each email into one of three statuses:

| Status | Meaning |
|---|---|
| `applied` | Initial application confirmation |
| `rejected` | Rejection received |
| `advanced` | Interview, offer, or next step |

Emails that are not job-related are silently discarded (`proceed: false`).

## Testing

```bash
task test        # unit + integration tests (requires Docker)
task lint        # golangci-lint
```

Integration tests use Testcontainers to spin up real Postgres and Kafka instances.

## CI/CD

GitHub Actions runs tests and lint on every push. On version tags (`v*`), it builds and pushes a Docker image to GitHub Container Registry.

```bash
docker build -t we-regret-to-persist:latest .
```
