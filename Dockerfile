# Build stage
FROM golang:1.23-alpine AS builder

WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o prepump ./cmd/prepump

# Final stage
FROM alpine:3.19

RUN apk add --no-cache ca-certificates

WORKDIR /app
COPY --from=builder /build/prepump .
COPY config.yaml .

# Run as non-root
RUN adduser -D -u 1000 appuser
USER appuser

ENTRYPOINT ["./prepump"]
