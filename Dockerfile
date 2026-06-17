FROM golang:1.25-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /forgec2 ./cmd/server

FROM alpine:3.19
RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app
COPY --from=builder /forgec2 /app/forgec2
COPY config.yaml /app/config.yaml

# Create data dirs
RUN mkdir -p /app/data/db /app/data/screenshots /app/data/agents

EXPOSE 8080

ENTRYPOINT ["/app/forgec2", "-config", "/app/config.yaml"]
