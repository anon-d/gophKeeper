# --- Build stage ---
FROM golang:1.26-alpine AS builder
RUN apk add --no-cache git
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /bin/server ./cmd/server
RUN CGO_ENABLED=0 go build -o /bin/migrator ./cmd/migrator

# --- Server image ---
FROM alpine:3.21 AS server
RUN apk add --no-cache ca-certificates
COPY --from=builder /bin/server /bin/server
ENTRYPOINT ["/bin/server"]

# --- Migrator image ---
FROM alpine:3.21 AS migrator
COPY --from=builder /bin/migrator /bin/migrator
COPY migrations /migrations
ENTRYPOINT ["/bin/migrator"]