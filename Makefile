SHELL := bash

.PHONY: run
run:
	go run ./cmd/server --port=8080

.PHONY: test
test:
	@go test -v -timeout=5s -vet=all -count=1 ./...

.PHONY: fmt
fmt:
	@goimports -local $(shell go list -m) -w .
	@gofumpt -l -w .

.PHONY: ensure-spdx
ensure-spdx:
	find . -type f -name "*.go" -exec sh -c 'head -1 {} | grep -q SPDX || echo "Missing SPDX on file {}"' \;
