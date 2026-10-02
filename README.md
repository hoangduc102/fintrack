# FinTrack

FinTrack is a full-stack personal finance web application built with **Go** and **React** for tracking income and expenses, planning category-level monthly budgets, analyzing cash flow, and importing transaction batches concurrently.

The application provides individuals with an isolated, privacy-first platform to monitor income vs. expenditure, track budget utilization in real time, and analyze historical spending trends without relying on closed commercial banking aggregators. The system is designed with a layered Go backend using raw SQL over PostgreSQL and a responsive React 19 single-page application.

---

## Features

### Authentication & User Isolation
- **User Registration & Login**: Account creation with automatic default category seeding; password hashing via `bcrypt`.
- **Stateless JWT**: 24-hour HMAC-SHA256 access tokens passed via standard Bearer headers.
- **Strict Data Scoping**: All database queries are explicitly scoped to the authenticated user ID (`WHERE user_id = $X`), ensuring total data isolation between accounts.

### Transactions
- **Income & Expense Tracking**: Records positive monetary values in Vietnamese Đồng (VND) with calendar dates (`YYYY-MM-DD`).
- **Flexible Categorization**: Transactions can belong to user-defined categories or remain Uncategorized.
- **Filtering & Pagination**: Server-side filtering by date range, transaction type (`INCOME` / `EXPENSE`), category, or uncategorized status, paired with deterministic offset pagination (`page`, `limit`).

### Monthly Budgets
- **Expense Limits**: Monthly spending limits set per expense category for any calendar month (`year`, `month`).
- **Upsert Semantics**: Supports creating or updating monthly budgets seamlessly (`ON CONFLICT DO UPDATE`).
- **Dynamic Spend Tracking**: Category spending is aggregated dynamically from actual expense transactions, preventing counter drift and synchronization bugs.

### Financial Analytics
- **Monthly Summary**: Aggregated total income, total expenses, net savings, and overall budget progress for any chosen month.
- **Expense Distribution**: Categorical expense breakdown showing exact amounts and percentage shares (including uncategorized spending).
- **Historical Cash-Flow Trend**: Rolling historical trend analysis (default 6 months, configurable from 1 to 24 months) showing monthly income, expenses, and savings with zero-filled continuous calendar intervals.

### CSV Batch Import
- **Multipart File Intake**: Uploads CSV transaction statements up to 5MB (`date,type,amount,category,description`).
- **Concurrent Validation**: Distributes row validation and category mapping across a 4-worker goroutine pool.
- **Atomic Batch Persistence**: Persists all valid rows in a single database transaction via prepared statements.
- **Partial Failure Reporting**: Captures row-level errors (line number and message) while importing valid records.

---

## Tech Stack

| Layer | Technology | Version | Purpose |
| :--- | :--- | :--- | :--- |
| **Backend Runtime** | [Go](https://go.dev/) | 1.25 | Core web service and concurrent execution engine |
| **HTTP Framework** | [Gin Gonic](https://github.com/gin-gonic/gin) | 1.10.0 | Routing, request binding, recovery, and middleware |
| **Database** | [PostgreSQL](https://www.postgresql.org/) | 16 (Alpine) | Relational persistence with foreign keys and check constraints |
| **Database Driver** | [jackc/pgx/v5](https://github.com/jackc/pgx) | 5.11.0 | High-performance PostgreSQL driver used via `database/sql` stdlib |
| **Authentication** | [golang-jwt/jwt/v5](https://github.com/golang-jwt/jwt) | 5.3.1 | Stateless HMAC-SHA256 JWT generation and claim verification |
| **Security** | [golang.org/x/crypto](https://pkg.go.dev/golang.org/x/crypto) | 0.23.0 | Adaptive password hashing with bcrypt |
| **Identifiers** | [google/uuid](https://github.com/google/uuid) | 1.6.0 | RFC 4122 UUIDv4 primary and foreign keys |
| **Configuration** | [joho/godotenv](https://github.com/joho/godotenv) | 1.5.1 | Local `.env` loading with environment variable fallback |
| **Schema Migrations** | [golang-migrate](https://github.com/golang-migrate/migrate) | v4.x | Explicit version-controlled DDL migration files |
| **Frontend Framework** | [React](https://react.dev/) | 19.2.8 | Declarative component UI |
| **Language** | [TypeScript](https://www.typescriptlang.org/) | ~6.0.2 | End-to-end client type safety |
| **Build Tool** | [Vite](https://vitejs.dev/) | 8.3.0 | Fast HMR dev server and production asset bundler |
| **Server State** | [TanStack Query](https://tanstack.com/query/latest) | 5.103.2 | Asynchronous query caching and mutation-driven cache invalidation |
| **Routing** | [React Router](https://reactrouter.com/) | 7.18.4 | Client-side routing with authentication guards |
| **Styling** | [Tailwind CSS](https://tailwindcss.com/) | 4.3.3 | Responsive utility styling with Vite integration |
| **Data Visualization** | [Recharts](https://recharts.org/) | 3.10.1 | Donut category breakdown and historical trend charts |
| **Icons** | [Lucide React](https://lucide.dev/) | 1.48.0 | UI icons |
| **Tooling / Container** | [Docker Compose](https://docs.docker.com/compose/) | v2.x | Local PostgreSQL containerization |

---

## Architecture

The backend follows a classic **layered architecture** with unidirectional dependency flow:

```mermaid
flowchart LR
    Client["Browser (React 19)"]
    Router["Gin HTTP Router"]
    MW["Auth Middleware"]
    Handler["Handler Layer"]
    Service["Service Layer"]
    Repo["Repository Layer"]
    DB[("PostgreSQL 16")]

    Client -->|HTTP / JSON| Router
    Router --> MW
    MW --> Handler
    Handler --> Service
    Service --> Repo
    Repo -->|database/sql + pgx| DB
```

### Layer Responsibilities

- **Handler (`internal/handler`)**: Parses HTTP requests, extracts route and query parameters, binds JSON bodies, validates transport-level formats, invokes service methods, and maps domain errors to standard HTTP status codes.
- **Service (`internal/service`)**: Contains business rules, cross-entity validation (e.g. verifying category type matches transaction type), password hashing and comparison, default category seeding, and concurrent worker-pool orchestration.
- **Repository (`internal/repository`)**: Defines Go persistence interfaces and implements data access using raw SQL with parameter placeholders (`$1, $2`). Manages database connection pooling, row scanning, and database transactions (`BeginTx`, `Commit`, `Rollback`).
- **Domain (`internal/domain`)**: Core domain structs (`User`, `Category`, `Transaction`, `Budget`, `MonthlySummary`) and sentinel error types (`ErrNotFound`, `ErrConflict`, `ErrUnauthorized`, `ValidationError`). Has zero external dependencies.
- **Middleware (`internal/handler/middleware.go`)**: Validates the `Authorization: Bearer <token>` header, parses JWT claims, and injects the authenticated `user_id` UUID into the `gin.Context`.

---

## Key Technical Highlights

### 1. Raw SQL with Connection Pooling (`database/sql` + `pgx`)
Rather than relying on ORMs that obscure generated queries, FinTrack uses Go's standard library `database/sql` backed by `jackc/pgx/v5/stdlib`. All queries use explicit parameter placeholders (`$1, $2`) to eliminate SQL injection risks. The connection pool is tuned for concurrent workloads (`MaxOpenConns: 25`, `MaxIdleConns: 25`, `ConnMaxLifetime: 15m`, `ConnMaxIdleTime: 5m`).

### 2. Whole-Integer Monetary Representation (VND)
All monetary amounts are stored as strictly positive 64-bit integers (`int64` in Go, `BIGINT` in PostgreSQL). Because Vietnamese Đồng does not employ fractional currency sub-units (cents), integer storage completely avoids floating-point precision loss (e.g. `0.1 + 0.2 != 0.3`) without the performance overhead or external dependencies of arbitrary-precision decimal libraries.

### 3. Enforced Tenant Data Scoping
Multi-tenancy is guaranteed directly at the database query boundary. The authenticated user ID extracted from the verified JWT is injected into every repository query (`WHERE user_id = $X`). Combined with composite indexes like `(user_id, transaction_date DESC)` and `(user_id, year, month)`, queries remain isolated and performant.

### 4. Concurrent CSV Ingestion via Worker Pool
The CSV import pipeline uses a 4-worker goroutine pool communicating over buffered channels (capacity 100). The producer reads rows sequentially, workers validate formats and map category names against an in-memory category lookup cache in parallel, and a coordinator synchronizes completion via `sync.WaitGroup`. Valid transactions are persisted in a single atomic database transaction using prepared statements, ensuring all-or-nothing database consistency while returning individual row validation errors.

### 5. Dynamic SQL Aggregation for Budgets and Analytics
Budget consumption and analytics summaries are calculated on the fly using SQL aggregation (`SUM`, `CASE WHEN`, `LEFT JOIN` within date boundaries) rather than stored in denormalized counter tables. This guarantees that importing, modifying, or deleting transactions immediately reflects in budget progress and monthly summaries without data drift.

### 6. Graceful Server Shutdown
The HTTP server uses `signal.NotifyContext` to capture OS termination signals (`SIGINT`, `SIGTERM`). When a signal is received, the server initiates a 10-second graceful shutdown window via `httpServer.Shutdown(ctx)`, allowing in-flight requests and database transactions to finish cleanly before closing the database connection pool.

---

## Database

FinTrack uses PostgreSQL 16 with explicit constraints, foreign key relationships, and query-optimized indexes.

```mermaid
erDiagram
    USERS ||--o{ CATEGORIES : "owns"
    USERS ||--o{ TRANSACTIONS : "records"
    USERS ||--o{ BUDGETS : "plans"
    CATEGORIES ||--o{ TRANSACTIONS : "classifies (ON DELETE SET NULL)"
    CATEGORIES ||--o{ BUDGETS : "targets (ON DELETE CASCADE)"

    USERS {
        uuid id PK
        varchar email UK
        varchar password_hash
        varchar display_name
        timestamptz created_at
        timestamptz updated_at
    }

    CATEGORIES {
        uuid id PK
        uuid user_id FK
        varchar name
        varchar type "CHECK IN (INCOME, EXPENSE)"
        timestamptz created_at
        timestamptz updated_at
    }

    TRANSACTIONS {
        uuid id PK
        uuid user_id FK
        bigint amount "CHECK (amount > 0)"
        varchar type "CHECK IN (INCOME, EXPENSE)"
        uuid category_id FK
        date transaction_date "YYYY-MM-DD"
        text description
        timestamptz created_at
        timestamptz updated_at
    }

    BUDGETS {
        uuid id PK
        uuid user_id FK
        uuid category_id FK
        bigint amount "CHECK (amount > 0)"
        int year "CHECK (2000-2100)"
        int month "CHECK (1-12)"
        timestamptz created_at
        timestamptz updated_at
    }
```

### Key Relationships & Constraints

- **Category Deletion Cascades**: When a category is deleted, referencing transactions have their `category_id` set to `NULL` (`ON DELETE SET NULL`) preserving financial history as Uncategorized. In contrast, monthly budgets targeting the deleted category are removed (`ON DELETE CASCADE`) because a budget cannot exist without an active category.
- **Unique Category Names**: `UNIQUE (user_id, type, LOWER(name))` prevents duplicate category names per user within the same category type.
- **Budget Uniqueness**: `UNIQUE (user_id, category_id, year, month)` ensures only one budget can be configured per category per month.
- **Calendar Dates**: `transaction_date` is stored as `DATE` (`YYYY-MM-DD`) without a time component, preventing timezone boundary shifts.

---

## API Overview

All application endpoints are versioned under `/api/v1` (with `/health` at the root). Protected endpoints require `Authorization: Bearer <token>`.

| Group | Method | Endpoint | Auth | Purpose |
| :--- | :--- | :--- | :---: | :--- |
| **Health** | `GET` | `/health` | No | System health check (`{"status":"ok"}`) |
| **Auth** | `POST` | `/api/v1/auth/register` | No | Register new user and seed standard categories |
| **Auth** | `POST` | `/api/v1/auth/login` | No | Authenticate credentials; returns 24h JWT token |
| **Auth** | `GET` | `/api/v1/auth/me` | Yes | Retrieve authenticated user profile |
| **Categories** | `GET` | `/api/v1/categories` | Yes | List categories (optional `?type=INCOME\|EXPENSE`) |
| **Categories** | `POST` | `/api/v1/categories` | Yes | Create custom category (`name`, `type`) |
| **Categories** | `PUT` | `/api/v1/categories/:id` | Yes | Rename existing category (`name`) |
| **Categories** | `DELETE`| `/api/v1/categories/:id` | Yes | Delete category (`ON DELETE SET NULL` on transactions) |
| **Transactions**| `POST` | `/api/v1/transactions` | Yes | Create single transaction (`amount`, `type`, `date`, etc.) |
| **Transactions**| `POST` | `/api/v1/transactions/import` | Yes | Upload CSV transaction file (multipart `file`, max 5MB) |
| **Transactions**| `GET` | `/api/v1/transactions` | Yes | List transactions with filters & offset pagination |
| **Transactions**| `GET` | `/api/v1/transactions/:id` | Yes | Retrieve single transaction by ID |
| **Transactions**| `PUT` | `/api/v1/transactions/:id` | Yes | Update existing transaction |
| **Transactions**| `DELETE`| `/api/v1/transactions/:id` | Yes | Delete transaction by ID |
| **Budgets** | `PUT` | `/api/v1/budgets` | Yes | Create or update budget (`amount`, `year`, `month`, `category_id`) |
| **Budgets** | `GET` | `/api/v1/budgets` | Yes | List budgets with dynamic spent amounts (`?month=YYYY-MM`) |
| **Budgets** | `DELETE`| `/api/v1/budgets/:id` | Yes | Delete budget by ID |
| **Analytics** | `GET` | `/api/v1/analytics/summary` | Yes | Monthly summary & budget progress (`?month=YYYY-MM`) |
| **Analytics** | `GET` | `/api/v1/analytics/categories` | Yes | Category expense distribution (`?month=YYYY-MM`) |
| **Analytics** | `GET` | `/api/v1/analytics/trend` | Yes | Rolling historical trend points (`?months=N`) |

### Example Payloads

#### Create Transaction (`POST /api/v1/transactions`)
```json
{
  "amount": 250000,
  "type": "EXPENSE",
  "category_id": "7b58713f-d31a-4d2c-8cb0-b0faef454f76",
  "transaction_date": "2026-10-02",
  "description": "Weekly groceries"
}
```

#### Monthly Analytics Summary (`GET /api/v1/analytics/summary?month=2026-10`)
```json
{
  "month": "2026-10",
  "total_income": 30000000,
  "total_expense": 14500000,
  "net_savings": 15500000,
  "budget_status": {
    "total_budget": 16000000,
    "total_spent": 14500000,
    "remaining": 1500000,
    "percentage": 90.63
  }
}
```

---

## Frontend

The frontend is a single-page application built with **React 19**, **TypeScript**, and **Vite**, styled with **Tailwind CSS v4**.

- **Routing & Authentication**: Built with React Router v7. An `AuthProvider` restores session state on initial page load by calling `GET /api/v1/auth/me`. Protected routes redirect unauthenticated users to `/login`.
- **Query Caching & Synchronization**: Uses TanStack Query v5 with a 5-minute default stale time. Mutations on transactions, categories, budgets, or CSV imports invalidate affected query keys (`['transactions']`, `['budgets']`, `['analytics']`), keeping the UI synchronized without full page reloads.
- **Dashboard & Visualizations**:
  - Four key metric cards: Total Income, Total Expenses, Net Savings, and Budget Usage percentage bar.
  - Category breakdown donut chart powered by **Recharts**.
  - Multi-month historical cash flow trend chart comparing income, expenses, and savings.
  - Inline monthly budget management panel for setting spending limits and viewing dynamic consumption.
- **Transactions Ledger**:
  - Filter bar supporting date ranges, transaction types, categories, and uncategorized toggles.
  - Offset pagination with direct page jumping.
  - Modal workflows for manual transaction creation/editing and CSV file imports.
- **Dev Reverse Proxy**: The Vite development server proxies `/api` requests to `http://localhost:8080`, mimicking production reverse-proxy routing and avoiding local cross-origin issues.

---

## CSV Import Pipeline

The CSV import pipeline demonstrates idiomatic Go concurrency for high-throughput batch intake:

```mermaid
flowchart TD
    Req["Client POST CSV (<= 5MB)"] --> Header["Preflight Header Check"]
    Header --> Cache["Preload User Categories Map (O(1) lookups)"]
    Cache --> Producer["Producer Goroutine: reads CSV rows -> jobs chan"]
    
    Producer --> Pool["4-Worker Goroutine Pool"]
    Pool -->|Validate & Map Category| Results["results chan"]
    
    Results --> Collector["Collector: gathers valid transactions & row errors"]
    Collector --> Tx["PostgreSQL Transaction (BeginTx)"]
    Tx --> Stmt["Batch Insert via Prepared Statement"]
    Stmt --> Commit["Commit & Return ImportResult"]
```

1. **Header Preflight**: Ensures headers match `date,type,amount,category,description` before worker startup.
2. **Category Preloading**: Pre-caches user categories into a map, eliminating $O(N)$ database round-trips.
3. **Channel Coordination**: Producer closes `jobs` on EOF; coordinator waits for workers with `sync.WaitGroup` and closes `results`.
4. **Context Propagation**: Workers monitor `ctx.Done()`, ensuring immediate resource release if the client disconnects.
5. **Partial Failures**: Invalid rows (malformed dates, non-positive amounts, invalid types) are reported with exact row numbers while valid rows are saved.

---

## Security & Data Isolation

- **Password Security**: Passwords hashed using `bcrypt` (default cost). The `PasswordHash` field in `domain.User` uses `json:"-"` to guarantee hashes are never serialized to JSON.
- **Stateless Tokens**: HMAC-SHA256 JWT tokens with a 24-hour expiration window.
- **Tenant Isolation**: Every SQL query filters by `WHERE user_id = $X`, preventing access to other users' records.
- **Request Limits**: CSV uploads are bounded to 5MB using `http.MaxBytesReader` to guard against denial-of-service memory exhaustion.
- **Validation**: Strict input validation on transaction amounts (positive integers), category types (`INCOME`/`EXPENSE`), and dates (`YYYY-MM-DD`).

---

## Local Development

### Prerequisites

- **Go**: Version `1.24+` (or `1.25`)
- **Node.js & pnpm**: Node `18+` and `pnpm` (or `npm`)
- **Docker & Docker Compose**: For containerized PostgreSQL
- **golang-migrate CLI**: For database migrations ([Installation guide](https://github.com/golang-migrate/migrate/tree/master/cmd/migrate))

---

### Step-by-Step Setup

#### 1. Clone Repository
```bash
git clone https://github.com/hoangduc102/fintrack.git
cd fintrack
```

#### 2. Start PostgreSQL Container
```bash
# Using Makefile
make db-up

# Or directly with Docker Compose
docker compose up -d
```

#### 3. Configure Environment Variables
Copy the example configuration to `.env`:

```bash
# Linux / macOS
cp .env.example .env

# Windows (PowerShell)
Copy-Item .env.example .env
```

#### 4. Run Database Migrations
```bash
# Using Makefile
make migrate-up

# Or with golang-migrate CLI directly
migrate -path migrations -database "postgres://fintrack:fintrack_secret@localhost:5432/fintrack?sslmode=disable" up
```

#### 5. Start Backend API Server
```bash
go run ./cmd/server
```
The server will start on port `8080`:
```
{"time":"...","level":"INFO","msg":"connected to database successfully"}
{"time":"...","level":"INFO","msg":"starting FinTrack HTTP server","port":"8080","env":"development"}
```

#### 6. Start Frontend Development Server
In a separate terminal:
```bash
cd web
pnpm install
pnpm run dev
```
Open `http://localhost:5173` in your browser.

---

## Environment Variables

FinTrack configuration is read from process environment variables, with optional auto-loading from `.env` in local development via `godotenv`:

| Variable | Required | Default | Description |
| :--- | :---: | :--- | :--- |
| `PORT` | No | `8080` | Port for the HTTP API server |
| `APP_ENV` | No | `development` | Environment mode (`development`, `test`, `production`) |
| `CORS_ALLOWED_ORIGINS` | No | `http://localhost:5173` | Comma-separated list of allowed CORS origins |
| `DATABASE_URL` | **Yes** | — | PostgreSQL connection string |
| `JWT_SECRET` | **Yes** | Dev fallback | 256-bit secret string for HMAC-SHA256 signing (must change in production) |

---

## Database Migrations

Schema migrations are managed via `golang-migrate` and stored under `migrations/`:

```
migrations/
├── 000001_init_schema.up.sql          # Creates users and categories tables
├── 000001_init_schema.down.sql
├── 000002_create_transactions.up.sql  # Creates transactions table and indexes
├── 000002_create_transactions.down.sql
├── 000003_create_budgets.up.sql       # Creates budgets table, indexes, and constraints
└── 000003_create_budgets.down.sql
```

**Commands:**
```bash
# Apply migrations
make migrate-up

# Roll back the latest migration
make migrate-down
```

---

## Testing

The project includes test coverage across unit, service, routing, concurrency, and database integration layers:

```bash
# Run all backend tests with the Go race detector enabled
make test

# Or directly:
go test -v -race ./...
```

### Test Categories

- **Unit & Logic Tests**:
  - `internal/auth`: Token generation, validation, signature verification, and expiration tests.
  - `internal/config`: Configuration parsing and fallback tests.
  - `internal/service`: Business rule validation and mock repository interactions.
- **Concurrency & Stress Tests**:
  - `internal/service/transaction_import_test.go`: Stress-tests the 4-worker CSV pipeline with 300 concurrent records, tests preflight validation, verifies partial error reporting, and validates clean shutdown on `context.Canceled`.
- **HTTP Routing & Middleware Tests**:
  - `internal/server/router_test.go`: Uses `net/http/httptest` to test every endpoint, status code, error envelope, and authentication requirement without requiring an active database.
  - `internal/server/server_test.go`: Tests server initialization and graceful shutdown signal handling.
- **Database Integration Tests**:
  - `internal/repository/*_test.go`: Integration tests executing real raw SQL against PostgreSQL (verifying atomicity, rollback on failure, and unique constraint violations). Automatically skipped if `DATABASE_URL` is unreachable.
- **Frontend Checks**:
  - Fast linting: `cd web && pnpm run lint` (`oxlint`)
  - Type checking & build: `cd web && pnpm run build` (`tsc -b && vite build`)

---

## Project Structure

```
fin-track/
├── cmd/
│   └── server/
│       └── main.go                 # HTTP server entry point & graceful shutdown
├── internal/
│   ├── auth/                       # JWT generation, token parsing, and validation
│   ├── config/                     # Typed configuration loading from environment
│   ├── database/                   # Connection pool configuration (pgx stdlib)
│   ├── domain/                     # Entities, models, and sentinel domain errors
│   ├── handler/                    # Gin HTTP route handlers and auth middleware
│   ├── repository/                 # PostgreSQL raw SQL data access implementations
│   ├── server/                     # Router assembly, options, and server lifecycle
│   └── service/                    # Business rules and concurrent CSV import engine
├── migrations/                     # Numbered up/down SQL schema migrations
├── web/                            # React 19 + TypeScript + Vite frontend
│   ├── src/
│   │   ├── api/                    # Typed API client and endpoint callers
│   │   ├── components/             # Reusable UI components, charts, and modals
│   │   ├── context/                # AuthContext session management
│   │   ├── pages/                  # Dashboard, Categories, Transactions pages
│   │   └── utils/                  # Currency (VND) formatting and date utilities
│   ├── package.json
│   └── vite.config.ts
├── docker-compose.yml              # PostgreSQL 16 container service
├── Makefile                        # Automation targets (db-up, migrate, test)
├── go.mod                          # Go module dependencies
├── go.sum                          # Go dependency checksums
└── .env.example                    # Template environment variables
```

---

## Deployment

FinTrack is designed for cloud-native deployment with decoupled frontend and backend services:

- **Frontend**: Compiles to static HTML/CSS/JS via `pnpm run build` for deployment on static hosting platforms (e.g. Vercel, Netlify, Cloudflare Pages).
- **Backend**: Containerized Go service exposing port `8080` on container runtimes (e.g. Render, Fly.io, Railway, or AWS ECS).
- **Database**: Managed PostgreSQL 16 instance.
- **Configuration**: Runtime configuration injected via platform environment variables (`DATABASE_URL`, `JWT_SECRET`, `PORT`, `CORS_ALLOWED_ORIGINS`).

> [!NOTE]
> Automated cloud deployment configuration and CI/CD pipelines are currently in progress.

---

## Engineering Trade-offs

1. **Integer VND vs. Floating-Point / Decimals**:
   - Storing VND as `int64` / `BIGINT` prevents rounding errors with zero performance overhead. This works cleanly because VND lacks sub-units, though multi-currency support in the future would require introducing currency codes and scaling factors.
2. **Raw SQL vs. ORMs**:
   - Writing raw SQL provides full query transparency, optimal index utilization, and precise control over transactions. The trade-off is writing manual row scanning code, which is mitigated by clear repository patterns.
3. **Synchronous Worker Pool vs. Distributed Job Queue**:
   - Ingesting CSV files via an in-process 4-worker goroutine pool gives users immediate feedback and partial failure reports within the HTTP request. This avoids the operational complexity of Redis/RabbitMQ, suitable for typical personal finance statement sizes (up to 5MB).
4. **Dynamic SQL Aggregation vs. Denormalized Summary Tables**:
   - Aggregating budgets and summaries on the fly guarantees 100% consistency across concurrent writes, edits, and imports. The trade-off is query computation on read, kept fast by composite indexes on `(user_id, transaction_date DESC)` and `(user_id, year, month)`.
5. **Deterministic Offset Pagination vs. Keyset Pagination**:
   - Offset pagination (`LIMIT $X OFFSET $Y`) allows users to see total item counts and jump to specific pages, aligning with user expectations for personal ledgers. If transaction volume grows to millions of rows per user, transitioning to keyset/cursor pagination would be preferred.
6. **Client-Side JWT Storage**:
   - Storing the access token in browser `localStorage` simplifies CORS and keeps the backend stateless, with the accepted trade-off of XSS vulnerability exposure typical for client-side SPAs.

---

## Screenshots / Demo

*(Screenshots and live demo link will be added upon deployment)*

---

## Roadmap

- [ ] **Refresh Token Rotation**: Short-lived access tokens paired with `httpOnly` refresh cookies for heightened session security.
- [ ] **Recurring Transactions**: Automated scheduled entry for monthly bills, subscriptions, and regular salaries.
- [ ] **Data Export**: Export filtered transactions and summary reports to CSV and Excel formats.
- [ ] **Custom Date Range Analytics**: User-selectable custom date comparison periods and expenditure velocity metrics.

---

## License

This project is currently unlicensed. All rights reserved by the author. You may review and evaluate the code for portfolio and assessment purposes.
