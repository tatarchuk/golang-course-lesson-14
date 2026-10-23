# albums-lambda: the album.submitted webhook of Lesson 12. `make help` lists the targets.
SHELL := /usr/bin/env bash

.DEFAULT_GOAL := help
.PHONY: help test build sam-build sam-invoke sam-api plan-tf deploy-tf destroy-tf bench

help: ## Show the targets
	@grep -E '^[a-zA-Z_-]+:.*## ' $(MAKEFILE_LIST) | awk -F':.*## ' '{printf "  %-12s %s\n", $$1, $$2}'

test: ## Unit tests (no Docker, no Floci)
	go vet ./...
	go test -count=1 -cover ./...

build: ## Build and pack the Lambda into build/webhook.zip
	scripts/build.sh

# sam build calls this target as `make build-<LogicalId>` with ARTIFACTS_DIR set (slides 20 and 42).
build-AlbumWebhookFunction:
	GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -tags lambda.norpc -ldflags="-s -w" -o $(ARTIFACTS_DIR)/bootstrap ./cmd/webhook

sam-build: ## sam build
	sam build

sam-invoke: sam-build ## sam local invoke with the valid, invalid and missing-field events
	sam local invoke AlbumWebhookFunction -e events/valid.json
	sam local invoke AlbumWebhookFunction -e events/invalid.json
	sam local invoke AlbumWebhookFunction -e events/missing_field.json

sam-api: sam-build ## sam local start-api on http://127.0.0.1:3000
	sam local start-api

plan-tf: build ## Terraform on Floci: init, validate and plan (read the diff)
	scripts/deploy-tf.sh plan

deploy-tf: build ## Terraform on Floci: apply, after the plan was read
	scripts/deploy-tf.sh apply

destroy-tf: build ## Terraform on Floci: destroy
	scripts/deploy-tf.sh destroy

bench: ## Cold vs warm calls of the deployed function on Floci (deck step 4)
	scripts/bench-cold-start.sh 10
