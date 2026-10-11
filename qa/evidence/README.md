# Test Execution Evidence Guide

This directory stores test execution evidence—including Postman response screenshots, JSON payloads, and terminal logs—supporting the test outcomes recorded in the FinTrack QA test suite.

---

## 1. Directory Structure

Evidence files are organized into subdirectories aligned with the core application modules:

```text
qa/evidence/
├── README.md               # Evidence management guidelines and index (this file)
├── auth/                   # Evidence for Authentication module test cases
├── transactions/           # Evidence for Transaction Management test cases
└── csv-import/             # Evidence for CSV Batch Import test cases
```

---

## 2. Verified Test Execution Evidence (Test Run `TR-001`)

The following **15 evidence files** were captured during Test Run `TR-001` on 2026-10-10, validating key positive, boundary, security, and error-handling test cases. All paths are relative to `qa/` and directly referenced in [qa/test-execution.csv](file:///d:/Go/pet-projects/fin-track/qa/test-execution.csv).

| Test Case ID | Evidence Relative Path | Description / Verified Behavior | File Status |
| :--- | :--- | :--- | :---: |
| **AUTH-001** | `evidence/auth/AUTH-001-register-success.png` | Successful registration returning `201 Created` with valid UUID and timestamps. | Verified on disk |
| **AUTH-008** | `evidence/auth/AUTH-008-password-least-8-characters.png` | Rejection of 7-character password returning `400 Bad Request` with field validation error. | Verified on disk |
| **AUTH-011** | `evidence/auth/AUTH-011-login-success.png` | Successful login returning `200 OK` and a signed 24-hour Bearer JWT token. | Verified on disk |
| **AUTH-015** | `evidence/auth/AUTH-015-missing-authorization-header.png` | Rejection of unauthenticated request returning `401 Unauthorized` (`missing authorization header`). | Verified on disk |
| **TRANS-001** | `evidence/transactions/TRANS-001.png` | Successful expense transaction creation returning `201 Created` with positive integer amount. | Verified on disk |
| **TRANS-006** | `evidence/transactions/TRANS-006.png` | Boundary verification for large whole integer amount (`50000000000` VND) returning `201 Created`. | Verified on disk |
| **TRANS-008** | `evidence/transactions/TRANS-008.png` | Category-type incompatibility rejection returning `400 Bad Request` (`category type does not match transaction type`). | Verified on disk |
| **TRANS-011** | `evidence/transactions/TRANS-011.png` | Rejection of empty transaction date returning `400 Bad Request` (`transaction_date is required`). | Verified on disk |
| **TRANS-012** | `evidence/transactions/TRANS-012.png` | Retrieval of transaction details by ID for the owner returning `200 OK`. | Verified on disk |
| **TRANS-013** | `evidence/transactions/TRANS-013.png` | IDOR prevention verification: User B accessing User A transaction returns masked `404 Not Found`. | Verified on disk |
| **TRANS-014** | `evidence/transactions/TRANS-014.png` | Successful transaction update via `PUT` returning `200 OK` with updated amount and description. | Verified on disk |
| **TRANS-015** | `evidence/transactions/TRANS-015.png` | Successful transaction deletion via `DELETE` returning `204 No Content`. | Verified on disk |
| **TRANS-016** | `evidence/transactions/TRANS-016.png` | Offset pagination and sorting verification returning `200 OK` with 20 items per page and `total_items`. | Verified on disk |
| **CSV-001** | `evidence/csv-import/CSV-001-valid-batch-import.png` | Successful CSV upload returning `200 OK` (`{"total": 5, "imported": 5, "failed": 0, "errors": null}`). | Verified on disk |
| **CSV-005** | `evidence/csv-import/CSV-005-mixed-valid-and-invalid-rows.png` | Partial batch intake returning `200 OK` with 3 imported rows and 3 line-numbered row validation errors. | Verified on disk |

---

## 3. Evidence Collection Standards & Naming Conventions

### File Naming Convention for Future Test Runs

To maintain consistency across testing cycles, name new evidence files using the following format:

$$\text{[TEST-CASE-ID]-[short-descriptive-slug].[ext]}$$

- **`[TEST-CASE-ID]`**: The test case identifier from [qa/test-cases.csv](file:///d:/Go/pet-projects/fin-track/qa/test-cases.csv) (e.g., `AUTH-002`, `TRANS-007`).
- **`[short-descriptive-slug]`**: Lowercase, hyphen-delimited summary of the verified scenario.
- **`.[ext]`**: File extension (`.png` for screenshots, `.json` for raw payloads, `.log` for terminal output).

#### Illustrative Examples *(Hypothetical future references)*:
- *Example only*: `evidence/auth/AUTH-003-duplicate-email-conflict.png`
- *Example only*: `evidence/transactions/TRANS-007-category-mismatch-validation.png`
- *Example only*: `evidence/csv-import/CSV-003-invalid-headers-rejected.png`

> [!IMPORTANT]
> The items under "Illustrative Examples" above represent formatting patterns for upcoming test runs. Only the files listed in Section 2 are physical artifacts currently stored in this repository.

---

## 4. Evidence Artifact Types

### 4.1. API Testing Evidence (Postman)
When capturing evidence from Postman:
1. Include the Request method and URL (`METHOD /path`).
2. Show the Request Headers and Request Body (if applicable).
3. Clearly display the HTTP Status Code and response time.
4. Show the Response Body.
5. Include the Postman **Test Results** tab showing passing assertion checks (`PASS: Status code is 201 Created`).

### 4.2. Database Verification Evidence
When validating database persistence or cascading rules:
- Run explicit queries against PostgreSQL via `psql` or Docker terminal:
  ```bash
  docker compose exec fintrack-postgres psql -U fintrack -d fintrack -c "SELECT id, amount, type, transaction_date FROM transactions WHERE user_id = '...' LIMIT 5;"
  ```
- Capture query output as formatted text or screenshot to confirm database state matches API responses.

---

## 5. Security & Privacy Safeguards

> [!CAUTION]
> **Strict Privacy Safeguards**:
> 1. **No Real Credentials**: Never use real email addresses, production passwords, or personal credentials during test execution. All test accounts must use synthetic local domains (`@fintrack.local`).
> 2. **Token Hygiene**: Bearer JWT tokens captured in screenshots or documentation belong strictly to local development instances running against transient test databases. Production secrets or long-lived production tokens must never be stored.
> 3. **Database Secrets**: Connection strings containing production passwords must never be captured in terminal logs or documentation.
