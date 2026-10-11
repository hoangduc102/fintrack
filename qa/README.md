# FinTrack QA Workspace & Testing Operations Guide

This workspace contains the complete Quality Assurance (QA) documentation, test case repository, Postman API testing assets, test execution logs, and verification evidence for the **FinTrack** application.

The testing architecture adheres to international software testing standards (ISTQB / IEEE 829), structured as a professional QA portfolio demonstrating manual API testing, positive/negative validation, security/IDOR boundary testing, and concurrency handling.

---

## 1. Directory Structure & File Inventory

The QA assets are organized into standardized modules within the `qa/` directory:

```text
qa/
├── README.md                      # QA Workspace operational guide (this file)
├── test-plan.md                   # Comprehensive Test Plan (scope, strategy, entry/exit criteria)
├── test-summary.md                # Test Execution Summary Report with real metrics (Run TR-001)
├── test-cases.csv                 # 46 standardized test cases (12-column TestRail schema)
├── test-execution.csv             # Execution log tracking actual results, statuses, and evidence
├── log-terminal-result-tests.txt  # Go backend automated test log (go test -v -race ./...)
├── bug-reports/                   # Central defect reporting repository
│   ├── README.md                  # Defect management lifecycle and reporting guidelines
│   └── BUG-REPORT-TEMPLATE.md     # Standardized defect report template (IEEE 829 format)
├── evidence/                      # Verification artifacts (screenshots, logs, payloads)
│   ├── README.md                  # Evidence index, naming conventions, and privacy rules
│   ├── auth/                      # Screenshots for Authentication test cases
│   ├── transactions/              # Screenshots for Transaction Management test cases
│   └── csv-import/                # Screenshots for CSV Batch Import test cases
├── postman/                       # Automated and semi-automated API testing assets
│   ├── README.md                  # Postman setup guide, execution order, and variables
│   ├── Fin-Track.postman_collection.json         # Postman Collection v2.1 with embedded tests
│   ├── FinTrack-Local.postman_environment.json  # Preconfigured local environment
│   └── environment.example.json                 # Environment variables template
└── test-data/                     # Synthetic test fixtures for CSV import validation
    ├── README.md                  # Fixture descriptions and 5MB boundary generation script
    ├── valid-transactions.csv     # 100% valid 5-row statement
    ├── mixed-transactions.csv     # Mixed batch with 3 valid and 3 invalid rows
    ├── invalid-headers.csv        # Localized/malformed column headers
    ├── empty-file.csv             # 0-byte empty file fixture
    ├── unknown-category.csv       # Unseeded category name fallback fixture
    └── category-type-mismatch.csv # Mismatched category type fallback fixture
```

---

## 2. QA Testing Workflow

The testing process follows a structured 6-stage lifecycle:

```mermaid
graph LR
    S1["1. Environment Setup"] --> S2["2. Postman Configuration"]
    S2 --> S3["3. Test Execution"]
    S3 --> S4["4. Log Execution in CSV"]
    S4 --> S5["5. Defect Management"]
    S5 --> S6["6. Summary & Sign-off"]
```

1. **Environment Setup**: Start the containerized PostgreSQL database and launch the Go backend service (and React frontend if testing UI integration).
2. **Postman Configuration**: Import the collection and environment into Postman Desktop; verify connectivity via `GET /health`.
3. **Test Execution**: Execute test cases sequentially from [qa/test-cases.csv](file:///d:/Go/pet-projects/fin-track/qa/test-cases.csv). Capture screenshots of responses and assertion checks.
4. **Log Execution in CSV**: Record execution date, tester name, actual response payload, and status (`PASS`, `FAIL`, `BLOCKED`, `NOT RUN`) into [qa/test-execution.csv](file:///d:/Go/pet-projects/fin-track/qa/test-execution.csv).
5. **Defect Management**: If an application defect is confirmed, create a new bug report under [qa/bug-reports/](file:///d:/Go/pet-projects/fin-track/qa/bug-reports/) using [BUG-REPORT-TEMPLATE.md](file:///d:/Go/pet-projects/fin-track/qa/bug-reports/BUG-REPORT-TEMPLATE.md), capture evidence, and reference the `Bug ID` in the execution log.
6. **Summary & Sign-off**: Compile execution metrics, pass rates, and findings into [qa/test-summary.md](file:///d:/Go/pet-projects/fin-track/qa/test-summary.md).

---

## 3. Environment Setup & Local Execution

Open a terminal (PowerShell on Windows, Bash on Linux/macOS) in the project root directory:

### 3.1. Start PostgreSQL Container
```powershell
# Start PostgreSQL container via Docker Compose
docker compose up -d

# Verify container health (status must be 'Up' and healthy on port 5432)
docker ps --filter "name=fintrack-postgres"
```

### 3.2. Apply Database Migrations (if not already applied)
```powershell
# Using Makefile
make migrate-up

# Or directly via golang-migrate CLI
migrate -path migrations -database "postgres://fintrack:fintrack_secret@localhost:5432/fintrack?sslmode=disable" up
```

### 3.3. Start the Go Backend Server
```powershell
go run ./cmd/server
```
*Expected log output*:
```text
{"time":"...","level":"INFO","msg":"connected to database successfully"}
{"time":"...","level":"INFO","msg":"starting FinTrack HTTP server","port":"8080","env":"development"}
```

### 3.4. Verify Backend Health
In a separate terminal:
```powershell
curl.exe -s http://localhost:8080/health
```
*Expected response*: `{"status":"ok"}`.

### 3.5. Start React Frontend (Optional for UI verification)
```powershell
cd web
pnpm install
pnpm run dev
```
Open `http://localhost:5173` in a web browser.

---

## 4. Postman API Testing Setup

### 4.1. Import Assets
1. Open Postman Desktop (v11.x).
2. Click **Import** (top-left) and select:
   - `qa/postman/Fin-Track.postman_collection.json`
   - `qa/postman/FinTrack-Local.postman_environment.json`
3. Activate the environment by selecting **`FinTrack Local`** from the environment dropdown in the top-right corner.

### 4.2. Recommended Execution Sequence
To satisfy authentication tokens and entity prerequisites:
1. **Health**: Execute `Health > Check Health` (`GET {{baseUrl}}/health`).
2. **Auth**: Run `Auth > Register` (User A), then `Auth > Login` (captures `{{token}}` automatically), then `Auth > Authorization` (`GET /api/v1/auth/me`).
3. **User B Setup**: Register and log in User B to capture `{{user_b_token}}` for IDOR verification.
4. **Categories**: Execute `Category CRUD > GET` to view the 6 default seeded categories and extract `{{categoryId}}`.
5. **Transactions**: Run `Transaction CRUD > CREATE` to generate `{{transactionId}}`, then test validation boundaries, IDOR protection, pagination, update, and delete.
6. **CSV Import**: Run `CSV Import > Import` using files from `qa/test-data/` (`valid-transactions.csv`, `mixed-transactions.csv`, etc.).

---

## 5. Differentiating Environmental Errors from Application Defects

| Observed Scenario | Underlying Root Cause | Classification | QA Action |
| :--- | :--- | :---: | :--- |
| `Connection Refused` or `connect ECONNREFUSED 127.0.0.1:8080` | Go server is not running or stopped unexpectedly. | **Environment Issue** | Start server via `go run ./cmd/server`. Mark test as `BLOCKED` in notes if unresolvable; do **not** file a bug report. |
| `HTTP 500` with log `failed to connect to postgres` | PostgreSQL container is offline or credentials in `.env` are mismatched. | **Environment Issue** | Run `docker compose up -d`. Mark test as `BLOCKED`. |
| Sending negative amount `-50000` returns `201 Created` instead of `400 Bad Request`. | Backend failed to enforce monetary constraints. | **Application Defect (Major)** | Capture screenshot to `qa/evidence/transactions/`, file bug report in `qa/bug-reports/`, mark test as `FAIL`. |
| User B Bearer token views User A transaction instead of receiving `404 Not Found`. | Broken Object Level Authorization (IDOR). | **Application Defect (Critical)** | File immediate `Critical` bug report with request/response evidence. |

---

## 6. Test Case Management Schema (TestRail Compatible)

The test case repository ([qa/test-cases.csv](file:///d:/Go/pet-projects/fin-track/qa/test-cases.csv)) is structured according to the 12-column **TestRail Test Case (Steps)** standard:

| Column | Description | Example |
| :--- | :--- | :--- |
| `Title` | Unique test case identifier and concise scenario name | `[AUTH-001] Verify successful registration...` |
| `Automation Type` | Automation status (`None` for manual API testing) | `None` |
| `Estimate` | Estimated time required to execute | `3m` |
| `Preconditions` | System and data prerequisites | `Backend running, email does not exist in DB` |
| `Priority` | Business criticality (`Critical`, `High`, `Medium`, `Low`) | `Critical` |
| `References` | HTTP endpoint and method under test | `POST /api/v1/auth/register` |
| `Section` | Functional module name | `Authentication` |
| `Section Description` | High-level overview of module scope | `User registration, auth, and isolation` |
| `Section Hierarchy` | Tree structure hierarchy for test management tools | `FinTrack > Authentication` |
| `Steps` | Step-by-step action instructions | `Send POST to /api/v1/auth/register with...` |
| `Expected Result` | Verification criteria and status codes | `HTTP 201 Created with UUIDv4 and timestamps` |
| `Type` | Testing methodology category | `Functional`, `Negative`, `Boundary`, `Security` |

---

## 7. Current Test Execution Baseline (Run `TR-001`)

The baseline manual API testing run was completed on **2026-10-10** with the following audited results:

- **Total Planned Test Cases**: 46
- **Executed Test Cases**: 45
- **Passed Test Cases**: 45
- **Failed Test Cases**: 0
- **Blocked Test Cases**: 0
- **Not Run Test Cases**: 1 (`AUTH-016`: non-Bearer authorization scheme validation)
- **Pass Rate among Executed**: **100.0%** ($\frac{45}{45}$)
- **Execution Rate**: **97.83%** ($\frac{45}{46}$)
- **Captured Evidence Artifacts**: 15 screenshots in [qa/evidence/](file:///d:/Go/pet-projects/fin-track/qa/evidence/)
- **Active Bug Reports**: 0 (no defects discovered during Run `TR-001`)

Detailed analysis, module-by-module observations, and recommendations are recorded in the [Test Execution Summary Report](file:///d:/Go/pet-projects/fin-track/qa/test-summary.md).
