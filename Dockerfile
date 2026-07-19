FROM golang:1.25-alpine AS builder
WORKDIR /build
RUN apk add --no-cache gcc musl-dev
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o forgec2-server ./cmd/server

FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata
RUN adduser -D -h /data forgec2
COPY --from=builder /build/forgec2-server /usr/local/bin/
EXPOSE 8080
USER forgec2
WORKDIR /data
ENTRYPOINT ["forgec2-server"]
CMD ["-config", "/data/config.yaml"]
