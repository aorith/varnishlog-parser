FROM golang:1.25 AS builder

WORKDIR /app

ARG VERSION=dev
RUN --mount=type=bind,target=. go mod download \
            && CGO_ENABLED=0 go build -ldflags="-X main.version=${VERSION}" -o /varnishlog-parser cmd/server/varnishlog-parser.go

FROM scratch

COPY --from=builder /varnishlog-parser /

USER 65534:65534

ENTRYPOINT ["/varnishlog-parser"]
CMD [""]
