# [BUG-XXX] Concise and Descriptive Defect Title

## 1. General Information

- **Bug ID**: `BUG-XXX` *(e.g., BUG-001)*
- **Title**: `[Module] Brief summary of unexpected behavior`
- **Module**: `Authentication` | `Transactions` | `CSV Import` | `Categories` | `Budgets` | `Analytics`
- **Related Test Case ID**: `AUTH-XXX` | `TRANS-XXX` | `CSV-XXX`
- **Reporter**: *(QA Tester Name)*
- **Report Date**: `YYYY-MM-DD`
- **Current Status**: `New` *(Allowed states: New, In Progress, Resolved, Verified, Closed, Reopened)*

---

## 2. Severity & Priority

### Severity: `Critical` | `Major` | `Medium` | `Minor`
> **Severity Classification Guidelines**:
> - **Critical**: System crash or panic (`HTTP 500`), financial data corruption or permanent data loss, cross-user data leakage or authorization bypass (IDOR / security vulnerability), or complete blocker of core flows without a workaround.
> - **Major**: Core business feature failure (e.g., unable to create a valid transaction, incorrect monetary calculations, login flow failure for valid credentials).
> - **Medium**: Exception handling defect, incomplete input validation, misleading or generic error messages, behavior deviating from specification where a viable workaround exists.
> - **Minor**: Cosmetic UI or layout alignment issues, typos, suboptimal date or currency formatting in non-critical displays.

### Priority: `High` | `Medium` | `Low`
> **Priority Scheduling Guidelines**:
> - **High**: Immediate developer attention required; must be resolved within the current day or before the next build release.
> - **Medium**: Fix required during the normal sprint cycle or upcoming scheduled patch.
> - **Low**: Low-impact refinement that can be addressed after core milestones are completed.

---

## 3. Environment & Application Version

- **Test Environment**: Local Development Environment
- **Service URLs**: Backend API (`http://localhost:8080`), Web Client (`http://localhost:5173`)
- **Application Version**: Git Commit HEAD (Go 1.25 / React 19)
- **Database**: PostgreSQL 16 (Docker Alpine container `fintrack-postgres`)
- **Testing Tool**: Postman Desktop v11.x / Google Chrome

---

## 4. Preconditions

- [ ] User A is registered and authenticated with a valid Bearer JWT access token.
- [ ] Required category records are seeded and available in the database.
- [ ] *(List any specific database states or configuration prerequisites)*

---

## 5. Steps to Reproduce

1. Send HTTP Request: `POST /api/v1/transactions`
2. Set Request Header: `Authorization: Bearer <valid_jwt_token>`
3. Set Request Body (JSON):
   ```json
   {
     "amount": 100000,
     "type": "EXPENSE",
     "category_id": "<income_category_uuid>",
     "transaction_date": "2026-10-09"
   }
   ```
4. Observe the HTTP response status code and response payload.

---

## 6. Expected vs. Actual Results

- **Expected Result**:
  - The API rejects the request with HTTP Status `400 Bad Request`.
  - The response body returns an error object:
    ```json
    {
      "error": {
        "code": "VALIDATION_ERROR",
        "fields": {
          "category_id": "category type does not match transaction type"
        },
        "message": "category type does not match transaction type"
      }
    }
    ```

- **Actual Result**:
  - The API responds with HTTP Status `500 Internal Server Error` (or creates the invalid record with mismatched types).

---

## 7. Reproduction Frequency

- [x] **Always (100%)**: Consistently reproducible on every execution.
- [ ] **Intermittent (<100%)**: Occurs intermittently (specify estimated frequency).
- [ ] **Single Occurrence**: Observed once; unable to reliably reproduce so far.

---

## 8. Evidence & Attachments

- **Evidence File Path**: `qa/evidence/transactions/TRANS-008_FAIL_type-mismatch-response.png` *(or relative JSON response file)*
- **Response Payload / Terminal Logs**:
  ```json
  // Attach relevant JSON response body or server log output here
  ```

---

## 9. Retest History

| Verification Date | Target Build / Commit | Tester | Retest Status | Notes |
| :---: | :---: | :---: | :---: | :--- |
| YYYY-MM-DD | Commit `<hash>` | *(QA Tester)* | `PASS` / `REOPEN` | *(Document regression test outcome)* |
