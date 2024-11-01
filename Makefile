watch:
	@templ generate -watch -cmd "go run . server --port=8088"

run:
	@go run . test

test:
	@go test -count=1 ./...

PHONY: watch run test
