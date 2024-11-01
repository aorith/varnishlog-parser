FROM golang:1.22 AS builder

WORKDIR /app

ARG VERSION="dev"
RUN --mount=type=bind,target=. go mod download \
            && CGO_ENABLED=0 go build -ldflags="-X cmd.Version=${VERSION}" -o /varnishlog-parser

FROM scratch

COPY --from=builder /varnishlog-parser /

ENTRYPOINT ["/varnishlog-parser"]
CMD ["server"]
