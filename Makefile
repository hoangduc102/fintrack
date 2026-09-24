DATABASE_URL ?= postgres://fintrack:fintrack_secret@localhost:5432/fintrack?sslmode=disable
DB_URL ?= $(DATABASE_URL)
MIGRATE ?= migrate

.PHONY: db-up db-down migrate-up migrate-down test

db-up:
	docker compose up -d

db-down:
	docker compose down

migrate-up:
	$(MIGRATE) -path migrations -database "$(DB_URL)" up

migrate-down:
	$(MIGRATE) -path migrations -database "$(DB_URL)" down 1

test:
	go test -v -race ./...
