# Internal Fund Transfers

A Go HTTP service for processing internal fund transfers between accounts, backed by PostgreSQL. Provides full ACID guarantees, deadlock-free concurrency via pessimistic row-level locking, and idempotent transaction processing.

## Features

- **Atomic transfers** — fund transfers with full ACID guarantees
- **Concurrency-safe** — pessimistic row-level locking with deadlock prevention via ordered lock acquisition
- **Idempotent transactions** — optional `Idempotency-Key` header prevents double-processing under retries
- **Decimal precision** — `NUMERIC(30,10)` in PostgreSQL, `shopspring/decimal` in Go — zero floating-point drift
- **Automatic retry** — exponential backoff with jitter on transient PostgreSQL errors
- **Structured logging** — Zap JSON logger with request-scoped `X-Request-ID`
- **Graceful shutdown** — SIGTERM drains in-flight requests with a 30s timeout

## Tech Stack


| Component        | Choice             |
| ---------------- | ------------------ |
| Language         | Go 1.23            |
| HTTP Router      | Gin v1.10          |
| ORM              | GORM v2            |
| Database         | PostgreSQL 16      |
| Decimal math     | shopspring/decimal |
| Config           | Viper (12-factor)  |
| Logging          | Zap (uber-go)      |
| Containerization | Docker + Compose   |


---

## Prerequisites

| Requirement | Version | Notes |
|-------------|---------|-------|
| Git | Any | For cloning the repository |
| Docker + Docker Compose | Engine 20.10+, Compose v2 | Required for Option A and B |
| Colima | Latest | macOS only, alternative to Docker Desktop |
| Go | 1.23+ | Required for Option C (local development) |
| PostgreSQL | 16+ | Required for Option C (local development) |
| make | Any | Required for Option C (`make run`, `make test`) |

> You only need the tools for your chosen setup method. For **Option A** (Docker Desktop) or **Option B** (Colima), Go and PostgreSQL are not required on your machine — everything runs inside containers.

---

## Installation

### 1. Clone the repository

```bash
git clone <repo-url>
cd internal-fund-transfers
```

### 2. Download Go dependencies

```bash
go mod download
go mod verify
```

> This step is optional for Docker-based setups (Options A and B) — dependencies are downloaded inside the container during `docker compose up --build`. It is required for Option C (local development) before running `make run` or `make test`.

---


## Setup

Choose **one** of the three methods below. All three result in the API running at `http://localhost:8080`.

- [Option A — Docker Desktop](#option-a--docker-desktop) — easiest, no manual PostgreSQL needed
- [Option B — Colima](#option-b--colima-macos) — Docker Desktop alternative for macOS
- [Option C — Manual PostgreSQL](#option-c--manual-postgresql) — run Go directly, bring your own PostgreSQL

---

## Option A — Docker Desktop

Docker Desktop bundles the Docker daemon, Docker Compose, and a GUI. Everything runs in containers — no separate PostgreSQL installation needed.

### Step 1 — Install Docker Desktop

**macOS**

1. Download Docker Desktop from **[https://www.docker.com/products/docker-desktop/](https://www.docker.com/products/docker-desktop/)**
  - Click **"Download for Mac"** → choose **Apple Silicon** (M1/M2/M3) or **Intel** based on your chip
2. Open the downloaded `.dmg` and drag **Docker** to the Applications folder
3. Open Docker from Applications
4. Accept the license agreement and complete the setup wizard (it will ask for your password to install helper tools)
5. Wait for the **whale icon** in the menu bar to stop animating — the daemon is ready

**Linux**

Follow the official instructions for your distro:

- Ubuntu/Debian: [https://docs.docker.com/engine/install/ubuntu/](https://docs.docker.com/engine/install/ubuntu/)
- After install, run: `sudo usermod -aG docker $USER` then log out and back in

**Windows**

Download and run the installer from [https://docs.docker.com/desktop/install/windows-install/](https://docs.docker.com/desktop/install/windows-install/). WSL 2 backend is recommended.

### Step 2 — Verify Docker is running

```bash
docker --version
# Docker version 27.x.x, build ...

docker compose version
# Docker Compose version v2.x.x

docker ps
# CONTAINER ID   IMAGE   COMMAND   CREATED   STATUS   PORTS   NAMES
```

If `docker ps` returns an error, Docker Desktop is not running — open it from Applications.

### Step 3 — Clone and start the service

```bash
# Clone the repository
git clone <repo-url>
cd internal-fund-transfers

# Build the image and start PostgreSQL + app
docker compose up --build -d
```

The first run downloads base images and compiles the Go binary — this takes ~2 minutes.

### Step 4 — Verify both services are healthy

```bash
docker compose ps
```

Expected output — both services should show `healthy`:

```
NAME                                    STATUS
internal-fund-transfers-postgres-1      running (healthy)
internal-fund-transfers-app-1           running (healthy)
```

If a service shows `starting`, wait 15 seconds and run `docker compose ps` again.

### Step 5 — Confirm the API is responding

```bash
curl http://localhost:8080/health
# {"status":"ok","db":"connected"}
```

### Step 6 — View logs

```bash
# All services
docker compose logs -f

# App only
docker compose logs -f app

# PostgreSQL only
docker compose logs -f postgres
```

### Managing the service

```bash
# Stop containers (keeps data volumes)
docker compose stop

# Stop and remove containers + volumes (wipes data)
docker compose down -v

# Restart after stopping
docker compose up -d

# Rebuild after code changes
docker compose up --build -d
```

---

## Option B — Colima (macOS)

Colima is a lightweight, open-source Docker runtime for macOS. It uses the same Docker CLI and Compose commands as Docker Desktop but has no GUI and lower resource overhead. Ideal if Docker Desktop won't start or you prefer a terminal-only setup.

### Step 1 — Install Homebrew (if not already installed)

```bash
/bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)"
```

Verify:

```bash
brew --version
# Homebrew 4.x.x
```

### Step 2 — Install Colima, Docker CLI, and plugins

```bash
brew install colima docker docker-compose docker-buildx
```

This installs:

- `colima` — the VM that runs the Docker daemon
- `docker` — the Docker CLI
- `docker-compose` — the Compose plugin
- `docker-buildx` — the BuildKit plugin (required for building images)

### Step 3 — Configure Docker plugins

Docker needs to know where Homebrew installed the plugins:

```bash
mkdir -p ~/.docker
cat > ~/.docker/config.json <<'EOF'
{
  "cliPluginsExtraDirs": [
    "/opt/homebrew/lib/docker/cli-plugins"
  ]
}
EOF
```

> **Note:** On Intel Macs, replace `/opt/homebrew` with `/usr/local`.

### Step 4 — Start Colima

```bash
colima start
```

This starts the Linux VM and the Docker daemon inside it. First run takes ~2 minutes to download the VM image. You should see:

```
INFO[...] starting colima
INFO[...] runtime: docker
INFO[...] creating and starting ...
INFO[...] READY. Run `limactl shell colima` to open the shell.
```

Verify the daemon is accessible:

```bash
docker ps
# CONTAINER ID   IMAGE   COMMAND   CREATED   STATUS   PORTS   NAMES
```

### Step 5 — Clone and start the service

```bash
# Clone the repository
git clone <repo-url>
cd internal-fund-transfers

# Build the image and start PostgreSQL + app
docker compose up --build -d
```

> If you see a **TLS handshake timeout** pulling images, the VM network is still initialising. Wait 10 seconds and try: `docker pull postgres:16-alpine` first, then re-run `docker compose up --build -d`.

### Step 6 — Verify both services are healthy

```bash
docker compose ps
```

Expected:

```
NAME                                    STATUS
internal-fund-transfers-postgres-1      running (healthy)
internal-fund-transfers-app-1           running (healthy)
```

### Step 7 — Confirm the API is responding

```bash
curl http://localhost:8080/health
# {"status":"ok","db":"connected"}
```

### Managing the service

```bash
# Stop containers (keeps data)
docker compose stop

# Stop containers and remove volumes (wipes data)
docker compose down -v

# Restart
docker compose up -d

# Stop Colima VM entirely (stops the Docker daemon)
colima stop

# Start Colima again next time
colima start
```

> Colima must be running before any `docker` or `docker compose` commands will work.

---

## Option C — Manual PostgreSQL

Run the Go binary directly on your machine with a locally installed PostgreSQL instance. Best for active development and debugging.

### Step 1 — Install Go 1.23

**macOS (Homebrew)**

```bash
brew install go@1.23
```

Add to your shell profile (`~/.zshrc` or `~/.bash_profile`):

```bash
export PATH="/opt/homebrew/opt/go@1.23/bin:$PATH"
```

Reload:

```bash
source ~/.zshrc
```

**Linux**

```bash
wget https://go.dev/dl/go1.23.4.linux-amd64.tar.gz
sudo tar -C /usr/local -xzf go1.23.4.linux-amd64.tar.gz
echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.bashrc
source ~/.bashrc
```

Verify:

```bash
go version
# go version go1.23.x ...
```

### Step 2 — Install PostgreSQL 16

**macOS (Homebrew)**

```bash
brew install postgresql@16
```

Start and enable PostgreSQL to run at login:

```bash
brew services start postgresql@16
```

Add the PostgreSQL binaries to your PATH:

```bash
echo 'export PATH="/opt/homebrew/opt/postgresql@16/bin:$PATH"' >> ~/.zshrc
source ~/.zshrc
```

**Linux (Ubuntu/Debian)**

```bash
sudo apt-get update
sudo apt-get install -y postgresql-16 postgresql-client-16

# Start the service
sudo systemctl start postgresql
sudo systemctl enable postgresql
```

Verify PostgreSQL is running:

```bash
pg_isready
# localhost:5432 - accepting connections
```

### Step 3 — Create the database

Connect to PostgreSQL and create the `transfers` database:

**macOS** (Homebrew installs PostgreSQL running as your OS user):

```bash
# Open the PostgreSQL prompt
psql postgres

# Inside psql:
CREATE DATABASE transfers;
\q
```

**Linux** (PostgreSQL runs as the `postgres` system user):

```bash
sudo -u postgres psql

# Inside psql:
CREATE DATABASE transfers;

-- Optional: set a password for the postgres user if you haven't already
ALTER USER postgres WITH PASSWORD 'postgres';

\q
```

Verify the database exists:

```bash
psql -U postgres -c "\l" | grep transfers
# transfers | postgres | UTF8 ...
```

### Step 4 — Configure environment variables

```bash
cp .env.example .env
```

Open `.env` and verify the values match your local PostgreSQL setup:

```dotenv
SERVER_PORT=8080

DB_HOST=localhost
DB_PORT=5432
DB_USER=postgres
DB_PASSWORD=postgres    # update if you set a different password
DB_NAME=transfers
DB_SSLMODE=disable
DB_MAX_OPEN=100
DB_MAX_IDLE=50
DB_MAX_LIFETIME=5m
DB_MAX_IDLE_TIME=1m

LOG_LEVEL=info

TRANSFER_MAX_RETRIES=3
```

> The `DB_PASSWORD` field is ignored when connecting to macOS Homebrew PostgreSQL without peer authentication. Leave it as `postgres`.

### Step 5 — Install make (if not already available)

**macOS:** `make` is included with Xcode Command Line Tools:

```bash
xcode-select --install
```

**Linux:**

```bash
sudo apt-get install -y make
```

### Step 6 — Build and run

The server reads configuration from **OS environment variables** (not the `.env` file directly). For a standard local PostgreSQL setup with defaults, just run:

```bash
make run
```

If your PostgreSQL credentials differ from the defaults, export them first:

```bash
export DB_USER=myuser
export DB_PASSWORD=mypassword
export DB_NAME=transfers
make run
```

Or source your `.env` file in one step:

```bash
export $(grep -v '^#' .env | xargs) && make run
```

You should see:

```json
{"level":"info","msg":"database connected","host":"localhost","port":5432,"dbname":"transfers"}
{"level":"info","msg":"server starting","addr":":8080"}
```

The schema is created automatically on first startup — no manual migration needed.

### Step 7 — Confirm the API is responding

Open a new terminal:

```bash
curl http://localhost:8080/health
# {"status":"ok","db":"connected"}
```

### Stopping the server

Press `Ctrl+C` in the terminal where the server is running. The server drains in-flight requests and exits cleanly.

---

## Smoke Test

Run these commands after any setup method to confirm the full request flow works end to end.

```bash
# 1. Health check
curl -s http://localhost:8080/health
# {"status":"ok","db":"connected"}

# 2. Create two accounts
curl -s -X POST http://localhost:8080/accounts \
  -H "Content-Type: application/json" \
  -d '{"account_id": 1, "initial_balance": "1000.00"}'
# {}

curl -s -X POST http://localhost:8080/accounts \
  -H "Content-Type: application/json" \
  -d '{"account_id": 2, "initial_balance": "500.00"}'
# {}

# 3. Check balances
curl -s http://localhost:8080/accounts/1
# {"account_id":1,"balance":"1000"}

curl -s http://localhost:8080/accounts/2
# {"account_id":2,"balance":"500"}

# 4. Transfer funds (with idempotency key)
curl -s -X POST http://localhost:8080/transactions \
  -H "Content-Type: application/json" \
  -H "Idempotency-Key: smoke-test-001" \
  -d '{"source_account_id": 1, "destination_account_id": 2, "amount": "250.00"}'
# {}

# 5. Verify balances updated
curl -s http://localhost:8080/accounts/1
# {"account_id":1,"balance":"750"}

curl -s http://localhost:8080/accounts/2
# {"account_id":2,"balance":"750"}

# 6. Replay the same transfer — balance must NOT change
curl -s -X POST http://localhost:8080/transactions \
  -H "Content-Type: application/json" \
  -H "Idempotency-Key: smoke-test-001" \
  -d '{"source_account_id": 1, "destination_account_id": 2, "amount": "250.00"}'
# {} (HTTP 200, not 201 — replay confirmed)

curl -s http://localhost:8080/accounts/1
# {"account_id":1,"balance":"750"}  ← unchanged
```

---

## Make Targets

```
make build            Build the binary to bin/server
make run              Build and run locally (requires .env and PostgreSQL)
make test             Run unit tests (no database required)
make test-integration Run integration + concurrency tests (requires PostgreSQL)
make test-cover       Run tests with coverage, generates coverage.html
make lint             Run golangci-lint
make docker-up        Build and start Docker stack in detached mode
make docker-down      Stop and remove containers and volumes
make docker-logs      Tail logs from all containers
make docker-status    Show container names and health status
make clean            Remove build artifacts and stop Docker containers
```

---

## API Reference

All requests and responses use `Content-Type: application/json`.

### POST /accounts

Create a new account with an initial balance.

**Request**

```bash
curl -s -X POST http://localhost:8080/accounts \
  -H "Content-Type: application/json" \
  -d '{"account_id": 1, "initial_balance": "1000.00"}'
```


| Field             | Type    | Required | Description                       |
| ----------------- | ------- | -------- | --------------------------------- |
| `account_id`      | integer | Yes      | Positive integer, client-provided |
| `initial_balance` | string  | Yes      | Decimal string, must be ≥ 0       |


**Response codes**


| Status                     | Meaning                                |
| -------------------------- | -------------------------------------- |
| `201 Created`              | Account created successfully           |
| `409 Conflict`             | An account with this ID already exists |
| `422 Unprocessable Entity` | Negative balance or non-numeric amount |
| `400 Bad Request`          | Malformed JSON body                    |


---

### GET /accounts/:id

Query an account balance.

```bash
curl -s http://localhost:8080/accounts/1
```

```json
{"account_id": 1, "balance": "1000"}
```


| Status            | Meaning                              |
| ----------------- | ------------------------------------ |
| `200 OK`          | Account found, balance returned      |
| `404 Not Found`   | Account does not exist               |
| `400 Bad Request` | Account ID is not a positive integer |


---

### POST /transactions

Transfer funds between two accounts.

The optional `Idempotency-Key` header makes the operation safe to retry — if the same key is seen again, the original response is returned without re-executing the transfer.

**Request**

```bash
curl -s -X POST http://localhost:8080/transactions \
  -H "Content-Type: application/json" \
  -H "Idempotency-Key: txn-abc-123" \
  -d '{
    "source_account_id": 1,
    "destination_account_id": 2,
    "amount": "100.50"
  }'
```


| Field                    | Type    | Required | Description                  |
| ------------------------ | ------- | -------- | ---------------------------- |
| `source_account_id`      | integer | Yes      | Must differ from destination |
| `destination_account_id` | integer | Yes      | Must differ from source      |
| `amount`                 | string  | Yes      | Decimal string, must be > 0  |


**Response codes**


| Status                     | Meaning                                                      |
| -------------------------- | ------------------------------------------------------------ |
| `201 Created`              | Transfer executed successfully                               |
| `200 OK`                   | Idempotent replay — key seen before, no re-execution         |
| `404 Not Found`            | Source or destination account does not exist                 |
| `422 Unprocessable Entity` | Insufficient funds, same-account transfer, or invalid amount |
| `400 Bad Request`          | Malformed JSON body                                          |


---

### GET /health

Liveness and database connectivity check.

```bash
curl -s http://localhost:8080/health
```

```json
{"status": "ok", "db": "connected"}
```

---

## Configuration

All configuration is via environment variables. Copy `.env.example` to `.env` for local development. The Docker Compose file sets these values directly in the `environment:` block.


| Variable               | Default     | Description                                       |
| ---------------------- | ----------- | ------------------------------------------------- |
| `SERVER_PORT`          | `8080`      | HTTP listen port                                  |
| `DB_HOST`              | `localhost` | PostgreSQL host                                   |
| `DB_PORT`              | `5432`      | PostgreSQL port                                   |
| `DB_USER`              | `postgres`  | Database user                                     |
| `DB_PASSWORD`          | `postgres`  | Database password                                 |
| `DB_NAME`              | `transfers` | Database name                                     |
| `DB_SSLMODE`           | `disable`   | SSL mode — `disable`, `require`, or `verify-full` |
| `DB_MAX_OPEN`          | `100`       | Maximum open connections in the pool              |
| `DB_MAX_IDLE`          | `50`        | Maximum idle connections in the pool              |
| `DB_MAX_LIFETIME`      | `5m`        | Maximum connection lifetime                       |
| `DB_MAX_IDLE_TIME`     | `1m`        | Maximum idle connection lifetime                  |
| `LOG_LEVEL`            | `info`      | Log level — `debug`, `info`, `warn`, or `error`   |
| `TRANSFER_MAX_RETRIES` | `3`         | Retry attempts for transient DB errors            |


---

## Testing

### Unit tests — no database required

```bash
make test
```

Tests the handler and service layers using in-memory stubs.

### Integration tests — requires PostgreSQL

The integration tests connect to a separate `transfers_test` database. The database must exist before running — the tests only create and drop the tables inside it, not the database itself.

**With Docker running (Option A or B):**

```bash
# Create the test database in the running PostgreSQL container
docker exec internal-fund-transfers-postgres-1 \
  psql -U postgres -c "CREATE DATABASE transfers_test;" 2>/dev/null || true

# Run integration tests against the containerised PostgreSQL
INTEGRATION_TEST=true \
DB_HOST=localhost DB_PORT=5432 \
DB_USER=postgres DB_PASSWORD=postgres \
DB_NAME=transfers_test \
  go test -race -count=1 -timeout 120s ./internal/...
```

**With local PostgreSQL (Option C):**

```bash
# Create the test database (only needed once)
psql -U postgres -c "CREATE DATABASE transfers_test;" 2>/dev/null || true

# Run integration tests (uses defaults from Viper — localhost:5432, postgres/postgres)
make test-integration

# If your credentials differ from defaults, export them first:
export DB_USER=myuser DB_PASSWORD=mypassword
make test-integration
```

### Coverage report

```bash
make test-cover
# Generates coverage.html — open in your browser
open coverage.html   # macOS
xdg-open coverage.html  # Linux
```

### What the integration suite covers

- Account lifecycle — create, duplicate rejection, balance query
- Transfer happy path and edge cases (insufficient funds, same account, decimal precision)
- **Concurrency** — 100 goroutines transferring from the same account simultaneously
- **Deadlock prevention** — circular transfer patterns (A→B, B→C, C→A) with 150 goroutines
- **Idempotency** — sequential and concurrent duplicate key handling (20 goroutines, same key)

---

## Architecture

```
cmd/server/main.go       Entry point — dependency wiring, graceful shutdown
internal/
├── config/              Viper-based 12-factor configuration
├── database/            GORM PostgreSQL connection + pool tuning
├── handler/             HTTP handlers (Gin) — request parsing and response mapping
├── middleware/          Logging (Zap), X-Request-ID injection
├── model/               GORM models — Account, Transaction, IdempotencyKey
├── repository/          Data access — SELECT FOR UPDATE, batch updates
├── service/             Business logic — transfer orchestration, idempotency
├── dto/                 Request / response types with validation tags
└── apperror/            Domain error sentinels
```

### Concurrency & Locking

Transfers acquire row-level exclusive locks (`SELECT ... FOR UPDATE`) on both account rows in a **single batch query ordered by ascending `account_id`**. This canonical lock ordering is a structural guarantee against deadlocks — no two transactions can form a lock cycle.

Transient PostgreSQL errors (serialization failure `40001`, deadlock detected `40P01`) trigger automatic retry with exponential backoff and random jitter to prevent thundering herd.

### Idempotency

`POST /transactions` accepts an optional `Idempotency-Key` header:

1. The key is looked up inside the transfer transaction using `SELECT FOR UPDATE SKIP LOCKED`
2. If the key exists and is readable, the stored response is returned immediately — transfer is not re-executed
3. If the key is locked by a concurrent commit (`SKIP LOCKED` skips it), the transfer proceeds, fails on `INSERT` with a unique constraint violation, rolls back, and re-fetches the committed key via a plain `SELECT`
4. Keys expire after 24 hours and are cleaned up by a background goroutine every 5 minutes

---

## Assumptions

1. All accounts use the same currency — no exchange rate logic
2. Account IDs are positive integers provided by the client
3. Amounts are JSON strings to preserve decimal precision (e.g. `"100.50"`)
4. Balance cannot go negative
5. No authentication or authorization required

