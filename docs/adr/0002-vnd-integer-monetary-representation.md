# Represent Money as Integer VND

We decided to model all monetary amounts as strictly positive 64-bit integers (`int64` in Go, `BIGINT` in PostgreSQL) representing whole Vietnamese Đồng (VND). This completely eliminates floating-point rounding errors and avoids external decimal library dependencies while matching the single-currency MVP scope.
