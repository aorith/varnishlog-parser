FROM golang:1.26 AS builder

WORKDIR /app

ARG VERSION=dev
# see 'go tool link' for ldflags
RUN --mount=type=bind,target=. go mod download \
            && CGO_ENABLED=0 go build -ldflags="-s -w -X main.version=${VERSION}" -o /varnishlog-parser cmd/server/varnishlog-parser.go

FROM scratch

COPY --from=builder /varnishlog-parser /

USER 65534:65534

ENTRYPOINT ["/varnishlog-parser"]
CMD [""]
