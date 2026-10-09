.PHONY: help up down db logs run test migrate-up migrate-down swagger measure er
.DEFAULT_GOAL := help

COMPOSE ?= docker compose
MIGRATE  = go run -tags postgres github.com/golang-migrate/migrate/v4/cmd/migrate@v4.17.1

# Host runs read their configuration from .env (copy .env.example first).
ifneq (,$(wildcard .env))
include .env
export
endif

help:          ## list the targets
	@grep -E "^[a-z-]+:.*## " Makefile | awk -F ":.*## " "{printf \"  %-14s %s\\n\", \$$1, \$$2}"

up:            ## build the image and start postgres + api
	$(COMPOSE) up -d --build

down:          ## stop the stack, keep the data volume
	$(COMPOSE) down

db:            ## start only postgres (for host runs and tests)
	$(COMPOSE) up -d postgres

logs:          ## follow the api logs (every SQL statement is logged)
	$(COMPOSE) logs -f api

run:           ## run the api on the host against the compose postgres
	go run ./cmd/api

test:          ## full suite with the race detector (uses TEST_DATABASE_URL from .env)
	go test ./... -race -count=1

migrate-up:    ## apply migrations to DATABASE_URL with the migrate CLI
	$(MIGRATE) -path migrations -database "$(DATABASE_URL)" up

migrate-down:  ## roll back every migration on DATABASE_URL
	$(MIGRATE) -path migrations -database "$(DATABASE_URL)" down -all

swagger:       ## regenerate docs/ from the handler annotations
	go run github.com/swaggo/swag/cmd/swag@v1.16.6 init -g cmd/api/main.go -o docs -ot go,yaml,json --parseInternal

measure:       ## N+1 evidence: SELECT count and latency of GET /api/v1/albums (api on the host, log in app.log)
	scripts/measure_n1.sh http://localhost:8080 app.log

er:            ## render docs/er.svg from docs/er.puml (needs plantuml)
	scripts/render_er.sh
