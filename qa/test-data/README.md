# CSV Import Test Data Fixtures

This directory contains test data fixtures designed for manual and automated validation of the FinTrack CSV Batch Import endpoint (`POST /api/v1/transactions/import`).

---

## 1. Test Data Fixture Inventory

Every test fixture is designed to validate specific preflight checks, worker-pool parsing rules, atomic persistence behavior, or category fallback semantics.

| File Name | Test Purpose | Expected Outcome | Associated Test Cases |
| :--- | :--- | :--- | :--- |
| **`valid-transactions.csv`** | Valid 5-row batch covering both `INCOME` and `EXPENSE` records with standard seeded categories. | `HTTP 200 OK`<br>`{"total": 5, "imported": 5, "failed": 0, "errors": null}`<br>All 5 transactions persisted in database. | `CSV-001`, `CSV-002` |
| **`mixed-transactions.csv`** | Mixed batch of 6 rows containing 3 valid transactions alongside 3 distinct validation errors (invalid date, invalid type, negative amount) and an unseeded category. | `HTTP 200 OK`<br>`{"total": 6, "imported": 3, "failed": 3, "errors": [...]}`<br>Valid rows committed atomically; invalid rows reported with 1-indexed row numbers. | `CSV-005` |
| **`invalid-headers.csv`** | File containing non-standard localized column headers (`ngay,loai,sotien,danhmuc,ghichu`). | `HTTP 400 Bad Request`<br>`"header must be date,type,amount,category,description"`<br>Preflight rejection; 0 rows processed. | `CSV-003` |
| **`empty-file.csv`** | 0-byte completely empty file. | `HTTP 400 Bad Request`<br>`"empty or invalid CSV file"`<br>Preflight rejection before worker initialization. | `CSV-004` |
| **`unknown-category.csv`** | Single transaction specifying an unseeded category name (`Cryptocurrency`). | `HTTP 200 OK`<br>Transaction is successfully imported with `category_id: null` (Uncategorized fallback per ADR-0008). | `CSV-006` |
| **`category-type-mismatch.csv`** | Single `EXPENSE` transaction referencing an `INCOME` category (`Salary`). | `HTTP 200 OK`<br>Transaction is successfully imported with `category_id: null` (Uncategorized fallback instead of rejecting row). | `CSV-007` |

---

## 2. Specification for FinTrack CSV Files

To pass preflight inspection, uploaded CSV files must adhere strictly to the following format:

### Required Header (Exact column names and order)
```csv
date,type,amount,category,description
```

### Field Constraints
1. **`date`**: ISO 8601 calendar date formatted as `YYYY-MM-DD` (e.g., `2026-10-09`). Values outside valid calendar ranges or using non-ISO formats (e.g., `DD/MM/YYYY`) fail row validation.
2. **`type`**: Must be either `INCOME` or `EXPENSE` (case-insensitive in import processing).
3. **`amount`**: Strictly positive whole integer in Vietnamese Đồng (`amount > 0`). Floating-point numbers, commas, and negative signs fail row validation.
4. **`category`**: Case-insensitive matching against the authenticated user's categories.
   - If the category exists and matches the transaction type, the transaction is assigned that category UUID.
   - If the category name is unknown or belongs to the opposite type, FinTrack automatically assigns `category_id: null` (`Uncategorized`) to ensure data preservation without failing the batch (ADR-0008).
5. **`description`**: Optional arbitrary text string.

---

## 3. Large File Boundary Testing (> 5MB Limit)

The backend handler enforces a **5MB payload ceiling** using Go's standard `http.MaxBytesReader` to protect against memory exhaustion attacks. To test this boundary without committing large binary or text files to the repository, generate a temporary synthetic CSV file using the following PowerShell command:

```powershell
# Run in PowerShell from the project root
$targetPath = "qa/test-data/oversized-transactions-temp.csv"
$header = "date,type,amount,category,description`n"
$row = "2026-10-09,EXPENSE,50000,Food & Dining,Boundary load test row data for 5MB limit verification`n"

[System.IO.File]::WriteAllText($targetPath, $header)
$stream = [System.IO.File]::OpenWrite($targetPath)
$stream.Seek(0, [System.IO.SeekOrigin]::End) | Out-Null
$writer = New-Object System.IO.StreamWriter($stream)

# Generate ~65,000 rows (~5.2 MB)
for ($i = 0; $i -lt 65000; $i++) {
    $writer.Write($row)
}
$writer.Flush()
$writer.Close()
$stream.Close()

Write-Host "Generated temporary oversized file: $((Get-Item $targetPath).Length / 1MB) MB"
```

### Verification & Cleanup
- When uploaded to `POST /api/v1/transactions/import`, the server will reject the request with HTTP Status `400 Bad Request` or terminate the connection early (`http: request body too large`).
- After completing the boundary test, remove the temporary file:
  ```powershell
  Remove-Item "qa/test-data/oversized-transactions-temp.csv"
  ```
