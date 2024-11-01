FROM golang:1.23 AS builder

WORKDIR /app

ARG VERSION="dev"
RUN --mount=type=bind,target=. go mod download \
            && CGO_ENABLED=0 go build -ldflags="-X cmd.Version=${VERSION}" -o /varnishlog-parser

FROM scratch

COPY --from=builder /varnishlog-parser /

USER 65534:65534

ENTRYPOINT ["/varnishlog-parser"]
CMD ["server"]
