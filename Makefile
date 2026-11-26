GOOSE_VERSION := v3.24.1
SQLC_VERSION := v1.29.0
OAPI_CODEGEN_VERSION := v2.8.2
GOOSE_TAGS := no_clickhouse no_libsql no_mssql no_mysql no_sqlite3 no_vertica no_ydb

.PHONY: generate clients codegen migrate migrate-status verify test compose-up compose-down

generate:
	cd apps/api && go run github.com/sqlc-dev/sqlc/cmd/sqlc@$(SQLC_VERSION) generate

clients:
	cd apps/api && go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@$(OAPI_CODEGEN_VERSION) -config oapi-codegen.yaml ../../contracts/externos/productos.yaml

codegen:
	pnpm codegen

migrate:
	cd apps/api && go run -tags='$(GOOSE_TAGS)' github.com/pressly/goose/v3/cmd/goose@$(GOOSE_VERSION) -dir db/migrations postgres "$(DATABASE_URL)" up

migrate-status:
	cd apps/api && go run -tags='$(GOOSE_TAGS)' github.com/pressly/goose/v3/cmd/goose@$(GOOSE_VERSION) -dir db/migrations postgres "$(DATABASE_URL)" status

verify:
	pnpm verify

test:
	pnpm test
	pnpm test:e2e:api

compose-up:
	docker compose up -d --build

compose-down:
	docker compose down
