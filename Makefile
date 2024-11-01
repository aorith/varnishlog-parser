watch:
	@templ generate -watch -cmd "go run . server --port=8080"

test:
	@go test -count=1 ./...

update_templ:
	@go install github.com/a-h/templ/cmd/templ@latest

PHONY: watch run update_templ
