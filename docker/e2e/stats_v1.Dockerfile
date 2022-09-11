FROM golang:1-alpine

WORKDIR /app

COPY go.mod ./
COPY go.sum ./
RUN go mod download

COPY cmd ./cmd
COPY pkg ./pkg
COPY internal ./internal

ENV CGO_ENABLED=0

ENTRYPOINT ["/usr/local/go/bin/go", "test", "-timeout", "30s", "./cmd/e2e_tests/stats_v1/..."]
