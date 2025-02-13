FROM golang:1.24 AS builder

WORKDIR /app

ARG VERSION=dev
RUN --mount=type=bind,target=. go mod download \
            && CGO_ENABLED=0 go build -ldflags="-X github.com/aorith/varnishlog-parser/cmd.Version=${VERSION}" -o /varnishlog-parser

FROM scratch

COPY --from=builder /varnishlog-parser /

USER 65534:65534

ENTRYPOINT ["/varnishlog-parser"]
CMD ["server"]
