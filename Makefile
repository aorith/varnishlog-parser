watch:
	@templ generate -watch -cmd "go run . server --port=8080"

test:
	@go test -timeout=5s -vet=all -count=1 ./...

update_templ:
	@go install github.com/a-h/templ/cmd/templ@latest

PHONY: watch test update_templ
