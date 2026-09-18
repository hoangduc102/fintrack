# Use Raw SQL with database/sql and pgx stdlib

We decided to use Go's standard library `database/sql` with `jackc/pgx/v5/stdlib` over ORMs (such as GORM) or query compilers (such as sqlc). Writing raw SQL queries and explicitly managing row scanning, error handling (`sql.ErrNoRows`), and database transactions maximizes Go fundamentals, prevents hidden behavior, and ensures high explainability in technical interviews.
