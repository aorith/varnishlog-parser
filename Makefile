PHONY: run test


run:
	go run . server --port=8080


test:
	@go test -v -timeout=5s -vet=all -count=1 ./...
