# Stage 1: Build Vite frontend
FROM node:20.18-alpine AS frontend-builder
WORKDIR /build/frontend
COPY frontend/package*.json ./
RUN npm ci
COPY frontend/ .
RUN npm run build

# Stage 2: Build Go binary with embedded frontend
FROM golang:1.25-alpine AS backend-builder
WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
# Copy frontend output into embed directory BEFORE copying source
COPY --from=frontend-builder /build/frontend/out /build/internal/webdist/dist
# Copy source code (local webdist/dist not needed in build context)
COPY cmd/ ./cmd/
COPY internal/ ./internal/
COPY pkg/ ./pkg/
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o forgec2-server ./cmd/server
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o healthcheck ./cmd/healthcheck
# Seed /data with nonroot ownership in the builder (which has a shell).
# NOTE: no RUN in the distroless stage — it has no shell and the build fails.
RUN mkdir -p /data/db && chown -R nonroot:nonroot /data

# Stage 3: Runtime — distroless for minimal attack surface
FROM gcr.io/distroless/static-debian12
LABEL org.opencontainers.image.title="ForgeC2"
LABEL org.opencontainers.image.description="Command & Control Framework"
COPY --from=backend-builder /build/forgec2-server /usr/local/bin/
COPY --from=backend-builder /build/healthcheck /usr/local/bin/healthcheck
# Docker copies image content (including ownership) into a fresh named volume,
# so the server can write its SQLite DB there despite running as nonroot.
# Without this, /data is root-owned and first startup crashes with
# "unable to open database file".
COPY --from=backend-builder --chown=nonroot:nonroot /data /data
EXPOSE 8000 443 8443 53
USER nonroot:nonroot
WORKDIR /home/nonroot
ENTRYPOINT ["forgec2-server"]
CMD ["-config", "/data/config.yaml"]
