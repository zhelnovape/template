-include .env
export

.PHONY: generate migrate run test

generate:
	go tool oapi-codegen -generate types,chi-server -package api -o internal/generated/api/api.gen.go contracts/openapi/trip-service.openapi.yaml
	go generate ./...

migrate:
	goose -dir migrations postgres "$(DATABASE_URL)" up

run:
	go run ./cmd/service

test:
	go test -v -race ./...