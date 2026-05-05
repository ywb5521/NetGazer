# ============================================================
# Stage 1: Build Go backend (server binary)
# ============================================================
FROM golang:1.24-bookworm AS backend-builder

RUN apt-get update && apt-get install -y --no-install-recommends \
    libpcap-dev libndpi-dev \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /src
COPY backend/go.mod backend/go.sum ./
RUN go mod download

COPY backend/ .
RUN CGO_ENABLED=1 go build -ldflags="-s -w -extldflags=-Wl,-rpath,\$ORIGIN,--disable-new-dtags" \
    -o /out/netgazer-server ./cmd/server

# ============================================================
# Stage 2: Build frontend
# ============================================================
FROM node:22-bookworm AS frontend-builder

WORKDIR /src
COPY frontend/package.json frontend/package-lock.json ./
RUN npm ci

COPY frontend/ .
RUN npm run build

# ============================================================
# Stage 3: Runtime
# ============================================================
FROM debian:bookworm-slim

RUN apt-get update && apt-get install -y --no-install-recommends \
    libpcap0.8 libndpi4.2 ca-certificates tzdata \
    && rm -rf /var/lib/apt/lists/* \
    && adduser --system --home /var/lib/netgazer --no-create-home netgazer

COPY --from=backend-builder /out/netgazer-server /usr/local/bin/netgazer-server
COPY --from=frontend-builder /src/dist /opt/netgazer/frontend/dist

RUN mkdir -p /var/lib/netgazer/geoip && \
    chown -R netgazer:netgazer /var/lib/netgazer /opt/netgazer

USER netgazer
EXPOSE 8080 50051
VOLUME ["/var/lib/netgazer"]

ENTRYPOINT ["netgazer-server"]
CMD ["--http-port", "8080", \
     "--grpc-port", "50051", \
     "--db", "/var/lib/netgazer/netgazer.db", \
     "--web-dir", "/opt/netgazer/frontend/dist"]
