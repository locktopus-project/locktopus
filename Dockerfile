FROM golang:1-alpine as build

WORKDIR /app

COPY go.mod ./
COPY go.sum ./
RUN go mod download

COPY cmd ./cmd
COPY pkg ./pkg
COPY internal ./internal

RUN go build -o ./bin/server ./cmd/server

# # #

FROM alpine:3

WORKDIR /

COPY --from=build /app/bin/server /bin/gearlock

ENTRYPOINT ["/bin/gearlock"]