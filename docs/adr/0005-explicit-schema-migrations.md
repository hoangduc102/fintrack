# Explicit Schema Migrations with golang-migrate

We decided to execute database migrations explicitly using `golang-migrate/migrate` via CLI/scripts or a dedicated container step, rather than automatically running migrations inside the API server on startup. Decoupling migrations from application boot prevents accidental schema alterations during server restarts and mirrors production practices where migrations are run as an isolated pre-deployment step.
