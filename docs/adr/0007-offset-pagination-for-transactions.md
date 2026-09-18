# Use Offset Pagination for Transaction History

We decided to implement offset-based pagination (`page` and `limit`) with deterministic ordering (`ORDER BY transaction_date DESC, created_at DESC, id DESC`) for `GET /api/v1/transactions`, rather than keyset or cursor-based pagination. While offset pagination has performance limitations at large scales, it supports random page jumping and total page counts in the UI, perfectly matching the expected data volume of personal financial tracking for the MVP while keeping implementation and testing straightforward.
