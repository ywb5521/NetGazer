# ============================================================
# Stage 1: Build Go backend (server binary)
# ============================================================
FROM golang:1.24-bookworm AS backend-builder

ENV GOTOOLCHAIN=auto

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
# Stage 3: Runtime (nginx + Go server, 单容器)
# ============================================================
FROM debian:bookworm-slim

RUN apt-get update && apt-get install -y --no-install-recommends \
    libpcap0.8 libndpi4.2 ca-certificates tzdata \
    nginx supervisor \
    && rm -rf /var/lib/apt/lists/* \
    && adduser --system --group --home /var/lib/netgazer --no-create-home netgazer \
    && mkdir -p /run/nginx

COPY --from=backend-builder /out/netgazer-server /usr/local/bin/netgazer-server
COPY --from=frontend-builder /src/dist /opt/netgazer/frontend/dist
COPY deploy/nginx/netgazer-docker.conf /etc/nginx/conf.d/default.conf
COPY deploy/supervisord.conf /etc/supervisor/supervisord.conf

RUN rm -f /etc/nginx/sites-enabled/default && \
    mkdir -p /var/lib/netgazer/geoip && \
    chown -R netgazer:netgazer /var/lib/netgazer /opt/netgazer

EXPOSE 9527
VOLUME ["/var/lib/netgazer"]

ENTRYPOINT ["/usr/bin/supervisord", "-c", "/etc/supervisor/supervisord.conf"]
