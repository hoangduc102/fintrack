# FinTrack QA Test Execution Summary Report

## 1. Executive Summary

- **Project Name**: FinTrack — Personal Finance Tracker
- **Test Run ID**: `TR-001` (Baseline Manual API Test Run)
- **Execution Date**: 2026-10-10
- **Lead QA Tester**: `hoangduc102`
- **Application Version**: Git Commit HEAD (Go 1.25 / React 19)
- **Environment**: Local Development (`http://localhost:8080`, PostgreSQL 16 Alpine Docker on port `5432`)
- **Primary Execution Source**: [qa/test-execution.csv](file:///d:/Go/pet-projects/fin-track/qa/test-execution.csv)
- **Test Case Repository**: [qa/test-cases.csv](file:///d:/Go/pet-projects/fin-track/qa/test-cases.csv)

This report summarizes the manual API test execution results for Test Run `TR-001`. The test session verified the functionality, input validation, authentication mechanisms, and data integrity across the three primary modules: **Authentication & Tenant Isolation**, **Transaction Management**, and **CSV Batch Import**.

All 45 executed test cases completed with a **PASS** result, achieving a **100.0% Pass Rate** among executed cases and a **97.83% Overall Test Coverage** across all 46 planned test cases. One designed test case (`AUTH-016`) was not executed during this session and is tracked as `NOT RUN`.

---

## 2. Test Execution Metrics

### 2.1. Overall Execution Results

| Metric | Count | Percentage | Formula / Definition |
| :--- | :---: | :---: | :--- |
| **Total Planned Test Cases** | **46** | 100.0% | Total unique test cases defined in `qa/test-cases.csv` |
| **Executed Test Cases** | **45** | 97.83% | $\frac{\text{Passed} + \text{Failed}}{\text{Total Planned}} = \frac{45}{46}$ |
| **Passed Test Cases** | **45** | 100.0% | Tests where actual result matched expected criteria |
| **Failed Test Cases** | **0** | 0.0% | Tests identifying confirmed application defects |
| **Blocked Test Cases** | **0** | 0.0% | Tests unable to proceed due to environment or blocking bugs |
| **Not Run Test Cases** | **1** | 2.17% | Tests designed in test repository but omitted from run (`AUTH-016`) |

### 2.2. Metric Formulas & Calculation Standards

1. **Execution Rate**:
   $$\text{Execution Rate} = \frac{\text{Executed Test Cases}}{\text{Total Planned Test Cases}} \times 100\% = \frac{45}{46} \times 100\% = 97.83\%$$

2. **Pass Rate (among Executed)**:
   $$\text{Pass Rate (Executed)} = \frac{\text{Passed}}{\text{Passed} + \text{Failed}} \times 100\% = \frac{45}{45 + 0} \times 100\% = 100.0\%$$

3. **Overall Pass Coverage**:
   $$\text{Overall Pass Coverage} = \frac{\text{Passed}}{\text{Total Planned Test Cases}} \times 100\% = \frac{45}{46} \times 100\% = 97.83\%$$

> [!NOTE]
> Initial draft planning documents estimated 54 test cases. Reconciliation against the authoritative test case repository ([qa/test-cases.csv](file:///d:/Go/pet-projects/fin-track/qa/test-cases.csv)) confirms exactly **46 unique test cases** (19 Authentication, 20 Transactions, 7 CSV Import). The metrics above strictly reflect this validated baseline.

---

## 3. Module Breakdown

| Module | Planned | Executed | Pass | Fail | Blocked | Not Run | Pass Rate (Executed) | Status |
| :--- | :---: | :---: | :---: | :---: | :---: | :---: | :---: | :---: |
| **1. Authentication (`AUTH-001` → `019`)** | 19 | 18 | 18 | 0 | 0 | 1 | 100.0% | Completed (`AUTH-016` deferred) |
| **2. Transactions (`TRANS-001` → `020`)** | 20 | 20 | 20 | 0 | 0 | 0 | 100.0% | Completed |
| **3. CSV Import (`CSV-001` → `007`)** | 7 | 7 | 7 | 0 | 0 | 0 | 100.0% | Completed |
| **Total** | **46** | **45** | **45** | **0** | **0** | **1** | **100.0%** | **Target Criteria Met** |

### 3.1. Detailed Module Observations

#### 1. Authentication & Tenant Isolation
- **Registration & Validation**: Account creation, duplicate email rejection (`409 Conflict`), case-insensitive email matching (`TESTER_A@FINTRACK.LOCAL`), and password boundary validations (7, 8, 72, and 73 bytes) performed exactly as specified.
- **Login & JWT Security**: Correct issuance of 24-hour Bearer tokens; rejection of non-existent emails and invalid passwords (`401 Unauthorized`).
- **Profile & Middleware**: Tampered JWT signatures, expired tokens, and missing authorization headers correctly returned `401 Unauthorized`.
- **Not Run Item (`AUTH-016`)**: `AUTH-016` ("Verify rejection of protected endpoint when Authorization header lacks Bearer scheme") was not manually executed in Test Run `TR-001`. Note: this scenario is validated by Go backend automated tests (`TestAuthMiddleware_MissingOrInvalidToken/invalid_header_format_without_bearer` in [qa/log-terminal-result-tests.txt](file:///d:/Go/pet-projects/fin-track/qa/log-terminal-result-tests.txt)).

#### 2. Transaction Management
- **Monetary & Date Handling**: Integer monetary values in VND and ISO calendar dates (`YYYY-MM-DD`) validated correctly. High-boundary amounts (`50,000,000,000` VND) were recorded and retrieved without precision loss or truncation.
- **Business Logic Rules**: Enforced category-to-transaction type compatibility (e.g., rejecting an Expense transaction referencing an Income category with `400 Bad Request`).
- **IDOR Prevention**: Verified that User B attempting to view User A's transaction received `HTTP 404 Not Found`, properly masking resource existence and preventing authorization bypass.
- **Ledger Operations**: Create, Read, Update, Delete, and offset pagination with descending date sorting verified without defects.

#### 3. CSV Batch Import
- **Worker-Pool Processing**: Concurrent 4-worker pipeline successfully parsed and validated incoming CSV statements.
- **Preflight Enforcement**: Localized headers and 0-byte empty files were rejected during preflight before worker initialization.
- **Partial Failure Handling**: Ingestion of mixed valid and invalid rows produced accurate row-level error reporting (rows 3, 4, 5) while saving valid records atomically.
- **Category Fallbacks**: Verified that unseeded categories (`unknown-category.csv`) and mismatched category types (`category-type-mismatch.csv`) gracefully defaulted to `category_id: null` (`Uncategorized`) per ADR-0008.

---

## 4. Defect Summary

| Defect ID | Severity | Module | Summary | Status | Test Case ID |
| :---: | :---: | :---: | :---: | :---: | :---: |
| — | — | — | *No application defects were detected during Test Run TR-001.* | — | — |

During this test execution cycle, all 45 executed requests met expected criteria. No defect reports were filed in [qa/bug-reports/](file:///d:/Go/pet-projects/fin-track/qa/bug-reports/).

---

## 5. Evidence Traceability

A total of **15 verification screenshots** were captured and stored in [qa/evidence/](file:///d:/Go/pet-projects/fin-track/qa/evidence/), confirming key positive, boundary, security, and error-handling outcomes:

- **Authentication (4 artifacts)**:
  - `evidence/auth/AUTH-001-register-success.png`
  - `evidence/auth/AUTH-008-password-least-8-characters.png`
  - `evidence/auth/AUTH-011-login-success.png`
  - `evidence/auth/AUTH-015-missing-authorization-header.png`
- **Transactions (9 artifacts)**:
  - `evidence/transactions/TRANS-001.png`
  - `evidence/transactions/TRANS-006.png`
  - `evidence/transactions/TRANS-008.png`
  - `evidence/transactions/TRANS-011.png`
  - `evidence/transactions/TRANS-012.png`
  - `evidence/transactions/TRANS-013.png` (IDOR masking verification)
  - `evidence/transactions/TRANS-014.png`
  - `evidence/transactions/TRANS-015.png`
  - `evidence/transactions/TRANS-016.png`
- **CSV Import (2 artifacts)**:
  - `evidence/csv-import/CSV-001-valid-batch-import.png`
  - `evidence/csv-import/CSV-005-mixed-valid-and-invalid-rows.png`

All 15 evidence paths are verified present on disk and linked directly in [qa/test-execution.csv](file:///d:/Go/pet-projects/fin-track/qa/test-execution.csv).

---

## 6. Key Technical Findings

1. **Multi-Tenant Security Is Robust**: Database query scoping (`WHERE user_id = $X`) consistently prevents cross-user information leakage. Unauthorized cross-tenant queries return `404 Not Found` rather than `403 Forbidden`, preventing resource enumeration.
2. **Whole-Integer Currency Prevents Arithmetic Drift**: Storing VND as `int64` / `BIGINT` guarantees exact calculations for large transactions (e.g., 50,000,000,000 VND) without floating-point inaccuracies.
3. **Resilient CSV Ingestion Pipeline**: The worker pool provides immediate feedback on partial row failures with exact line numbers while preserving transaction atomicity for valid records.
4. **Automated Backend Test Coverage**: Go backend automated test suites (29 test suites across unit, service, repository, routing, and concurrency layers) achieved 100% pass rates with the Go race detector enabled (`go test -v -race ./...`), validating concurrency safety.

---

## 7. Known Limitations & Recommendations

1. **Execute Deferred Test Case `AUTH-016`**: Schedule manual execution of `AUTH-016` in Test Run `TR-002` to confirm non-Bearer header rejection in Postman.
2. **Expand Coverage to Budgets & Analytics (Phase 2)**: Design and execute manual test suites for the Budgets (`/api/v1/budgets`) and Analytics (`/api/v1/analytics`) modules to complete functional API test coverage.
3. **Database Reset Scripting**: Implement an automated database reset / seeding script between test runs to avoid manual cleanup of accumulated transaction and user records.
4. **Postman Runner Integration**: Leverage Newman (CLI runner) in CI/CD pipelines to run the Postman collection automatically upon pull request creation.

---

## 8. Final Assessment & Sign-Off

The FinTrack core API implementation (Authentication, Transactions, CSV Import) demonstrates high stability, strict adherence to domain specifications, and robust tenant isolation. Test Run `TR-001` satisfies all functional exit criteria for Phase 1.
