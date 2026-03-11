# we-regret-to-persist

[![CI](https://github.com/sim-pez/we-regret-to-persist/actions/workflows/ci.yml/badge.svg)](https://github.com/sim-pez/we-regret-to-persist/actions/workflows/ci.yml)
[![Go](https://img.shields.io/badge/go-1.24-blue?logo=go)](https://go.dev)
[![Claude](https://img.shields.io/badge/Claude-Haiku%204.5-blueviolet?logo=anthropic)](https://www.anthropic.com)
[![Docker](https://img.shields.io/badge/GHCR-ghcr.io%2Fsim--pez%2Fwe--regret--to--persist-blue?logo=docker)](https://github.com/sim-pez/we-regret-to-persist/pkgs/container/we-regret-to-persist)

An AI-powered job application tracker that automatically processes emails and maintains a database of your application statuses — so you never lose track of the rejections you'd rather forget.

## Overview

This service consumes job-related emails from a Kafka topic, uses Claude AI to classify each one, and upserts the result into PostgreSQL. It demonstrates a clean event-driven architecture with a focused use case: turning an inbox full of recruiter noise into a structured, queryable dataset.

## We regret stack
This is a service in the "We regret" stack, a collection of services that process recruiter emails and maintain a comprehensive job application history.

The stack is as follows:
- **n8n** — monitors your email inbox, extracts relevant emails, and produces them to the `emails` Kafka topic
- **we-regret-to-persist** (this service) — processes application confirmations and rejections, maintains the source of truth in Postgres
- **postgres** — serves as the single source of truth for all application statuses and statistics (e.g. total applications, rejection rate, timeline of events)
- **we-regret-to-present** — serves a REST API to query your application history


## Tech stack

| Layer | Technology |
|---|---|
| Language | Go 1.24 |
| AI | Claude Haiku 4.5 (`anthropic-sdk-go`) |
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
