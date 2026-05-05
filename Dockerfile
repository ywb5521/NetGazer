# ============================================================
# Stage 1: Build Go backend (server binary)
# ============================================================
FROM golang:1.25-alpine AS backend-builder

RUN apk add --no-cache gcc musl-dev libpcap-dev

WORKDIR /src
COPY backend/go.mod backend/go.sum ./
RUN go mod download

COPY backend/ .
RUN CGO_ENABLED=1 go build -ldflags="-s -w" -o /out/netgazer-server ./cmd/server

# ============================================================
# Stage 2: Build frontend
# ============================================================
FROM node:22-alpine AS frontend-builder

WORKDIR /src
COPY frontend/package.json frontend/package-lock.json ./
RUN npm ci

COPY frontend/ .
RUN npm run build

# ============================================================
# Stage 3: Runtime
# ============================================================
FROM alpine:3.22

RUN apk add --no-cache libpcap ca-certificates tzdata && \
    adduser -D -h /var/lib/netgazer netgazer

COPY --from=backend-builder /out/netgazer-server /usr/local/bin/netgazer-server
COPY --from=frontend-builder /src/dist /opt/netgazer/frontend

RUN mkdir -p /var/lib/netgazer/geoip && \
    chown -R netgazer:netgazer /var/lib/netgazer /opt/netgazer

USER netgazer
EXPOSE 8080 50051
VOLUME ["/var/lib/netgazer"]

ENTRYPOINT ["netgazer-server"]
CMD ["--http-port", "8080", "--grpc-port", "50051", "--db", "/var/lib/netgazer/netgazer.db"]
