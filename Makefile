-include .env
export

.PHONY: generate migrate run lint test test-integration

generate:
	go tool oapi-codegen -generate types,chi-server -package api -o internal/generated/api/api.gen.go contracts/openapi/trip-service.openapi.yaml
	go generate ./...

migrate:
	goose -dir migrations postgres "$(DATABASE_URL)" up

run:
	go run ./cmd/service

lint:
	golangci-lint run ./...

test:
	go test -race -count=1 ./...
	go test -race -count=1 -coverprofile=coverage.out ./internal/domain/... ./internal/usecase/...
	go tool cover -func=coverage.out | tail -n 1

test-integration:
	go test -race -count=1 -tags=integration ./internal/repository/...
