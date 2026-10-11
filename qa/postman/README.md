# Postman API Testing Suite Guide

This directory provides the Postman collection and environment configurations used for manual and automated API verification of the FinTrack REST API.

---

## 1. File Inventory

- [Fin-Track.postman_collection.json](file:///d:/Go/pet-projects/fin-track/qa/postman/Fin-Track.postman_collection.json): Comprehensive Postman Collection (v2.1) organized by functional module with pre-request scripts, embedded assertions (`pm.test`), and status code validations.
- [FinTrack-Local.postman_environment.json](file:///d:/Go/pet-projects/fin-track/qa/postman/FinTrack-Local.postman_environment.json): Pre-configured local testing environment defining `baseUrl` (`http://localhost:8080`), authentication tokens, and dynamic entity IDs.
- [environment.example.json](file:///d:/Go/pet-projects/fin-track/qa/postman/environment.example.json): Template environment file documenting all supported variable keys (`base_url`, `test_email`, `token`, `user_b_token`, etc.) for custom test runners.

---

## 2. Setup & Installation in Postman

### Step 1: Launch Postman
Open Postman Desktop (v11.x recommended).

### Step 2: Import Collection and Environment
1. Click the **Import** button in the top-left corner of Postman.
2. Select and upload the two configuration files:
   - `qa/postman/Fin-Track.postman_collection.json`
   - `qa/postman/FinTrack-Local.postman_environment.json`
3. Click **Import** to load them into your workspace.

### Step 3: Activate the Environment
1. In the upper-right corner of Postman, locate the environment dropdown (initially *No Environment*).
2. Select **`FinTrack Local`** (or **`FinTrack Local Dev (Example)`**).
3. Verify that `baseUrl` is configured to `http://localhost:8080`.

---

## 3. Dynamic Variables & Workflow Automation

The Postman test scripts dynamically capture and propagate tokens and entity identifiers across consecutive requests, eliminating manual copy-pasting:

1. **Authentication Token Propagation**:
   - Executing `Auth > Login` sends valid user credentials.
   - The test script extracts `response.token` and automatically saves it into the active environment variable `token`.
   - All subsequent authenticated requests reference `Authorization: Bearer {{token}}`.
2. **Category UUID Extraction**:
   - Calling `Category CRUD > GET` lists user-owned categories.
   - The script or tester captures a seeded category UUID and assigns it to `{{categoryId}}`.
3. **Transaction ID Tracking**:
   - Calling `Transaction CRUD > CREATE` receives `response.id`.
   - The script sets `{{transactionId}}` for downstream `GET /:id`, `PUT /:id`, and `DELETE /:id` validation.

---

## 4. Recommended Execution Order

To satisfy entity dependencies and preconditions during a manual or runner session, execute requests in the following sequence:

```text
1. System Health Check
   └─ GET {{baseUrl}}/health                      (Verifies server readiness: 200 OK)

2. Authentication & Authorization
   ├─ POST {{baseUrl}}/api/v1/auth/register       (Register User A: tester_a@fintrack.local)
   ├─ POST {{baseUrl}}/api/v1/auth/login          (Login User A; automatically captures {{token}})
   ├─ GET  {{baseUrl}}/api/v1/auth/me             (Validate User A profile & JWT authenticity)
   └─ Register & Login User B                     (Captures {{user_b_token}} for IDOR testing)

3. Categories
   ├─ GET  {{baseUrl}}/api/v1/categories          (Verify default 6 seeded categories)
   └─ POST {{baseUrl}}/api/v1/categories          (Test custom category creation)

4. Transaction Management
   ├─ POST {{baseUrl}}/api/v1/transactions        (Positive creation; captures {{transactionId}})
   ├─ Validation Tests (Amounts / Dates / Types)   (Verify 400 Bad Request responses)
   ├─ GET  {{baseUrl}}/api/v1/transactions/:id    (Verify retrieval by ID for owner)
   ├─ IDOR Security Verification                  (User B token requesting User A ID -> 404)
   ├─ GET  {{baseUrl}}/api/v1/transactions        (Test pagination & filters: type, dates)
   ├─ PUT  {{baseUrl}}/api/v1/transactions/:id    (Verify transaction update)
   └─ DELETE {{baseUrl}}/api/v1/transactions/:id (Verify deletion; returns 204 No Content)

5. CSV Batch Import
   ├─ POST {{baseUrl}}/api/v1/transactions/import (valid-transactions.csv -> 5 imported)
   ├─ POST {{baseUrl}}/api/v1/transactions/import (invalid-headers.csv -> 400 rejected)
   ├─ POST {{baseUrl}}/api/v1/transactions/import (empty-file.csv -> 400 rejected)
   └─ POST {{baseUrl}}/api/v1/transactions/import (mixed-transactions.csv -> partial failure report)

6. Budgets & Analytics
   ├─ PUT  {{baseUrl}}/api/v1/budgets             (Upsert monthly budget for expense category)
   ├─ GET  {{baseUrl}}/api/v1/budgets             (List monthly budgets with dynamic spend)
   ├─ GET  {{baseUrl}}/api/v1/analytics/summary   (Monthly income, expense, savings summary)
   ├─ GET  {{baseUrl}}/api/v1/analytics/categories(Category distribution breakdown)
   └─ GET  {{baseUrl}}/api/v1/analytics/trend     (Multi-month historical trend)
```

---

## 5. Destructive Request Handling

> [!WARNING]
> The `DELETE /api/v1/transactions/{{transactionId}}` endpoint permanently removes the transaction from PostgreSQL.
> - **During Manual Testing**: Execute DELETE test cases only after completing all dependent read, filter, and update checks.
> - **In Automated Collection Runner**: Uncheck the DELETE transaction request if subsequent dashboard or ledger verification requires existing transaction history.

---

## 6. Exporting Test Results and Capturing Evidence

1. **Using Postman Collection Runner**:
   - Right-click the `Fin-Track` collection in the sidebar and select **Run collection**.
   - Select the target folders and set the environment to **FinTrack Local**.
   - Click **Run FinTrack**.
2. **Exporting Evidence**:
   - When the run completes, click **Export Results** to save the execution summary as a JSON report.
   - For visual portfolio evidence, capture a screenshot of the Postman runner summary showing the passed assertions (`Status code is 201 Created`, `Valid JSON schema`, etc.).
   - Save the screenshot to the appropriate subdirectory in [qa/evidence/](file:///d:/Go/pet-projects/fin-track/qa/evidence/) following the project naming convention.
