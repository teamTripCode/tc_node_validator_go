# Build stage
FROM golang:1.24-alpine AS builder

# Instalar dependencias necesarias
RUN apk add --no-cache git ca-certificates tzdata

# Crear usuario no-root
RUN adduser -D -s /bin/sh -u 1000 appuser

WORKDIR /app

# Copiar archivos de módulos Go primero (para aprovechar cache de Docker)
COPY go.mod go.sum ./
RUN go mod download

# Copiar todo el código fuente
COPY . .

# Build con optimizaciones
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags='-w -s -extldflags "-static"' \
    -a -installsuffix cgo \
    -o /dpos-node .

# Final stage
FROM alpine:3.19

# Instalar dependencias mínimas
RUN apk add --no-cache ca-certificates tzdata curl

# Crear usuario no-root con el mismo UID que el StatefulSet
RUN adduser -D -s /bin/sh -u 1000 appuser

# Crear directorios necesarios con permisos correctos
# /app/data y /app/logs serán manejados por VOLUMEs
RUN mkdir -p /app/tmp \
             /app/config \
             /etc/tripcodechain/mcp && \
    chown -R appuser:appuser /app /etc/tripcodechain && \
    chmod -R 755 /app /etc/tripcodechain && \
    chmod -R 777 /app/tmp

# Copiar binario
COPY --from=builder /dpos-node /usr/local/bin/dpos-node
RUN chmod +x /usr/local/bin/dpos-node

# Cambiar a usuario no-root
USER appuser

# Establecer directorio de trabajo
WORKDIR /app

# Variables de entorno por defecto (sin datos sensibles)
ENV DATA_DIR=/app/data \
    LOG_DIR=/app/logs \
    CONFIG_DIR=/app/config \
    P2P_PORT=3001 \
    API_PORT=3002 \
    METRICS_PORT=9090 \
    PBFT_WS_PORT=8546 \
    PBOS_WS_PORT=8547 \
    SEED_NODES="" \
    NODE_TYPE="validator" \
    BOOTSTRAP_PEERS="" \
    IP_SCANNER_ENABLED="false" \
    IP_SCAN_RANGES="127.0.0.1/24" \
    IP_SCAN_TARGET_PORT="3001"

# Puertos a exponer:
# P2P_PORT (HTTP para algunas interacciones, el real de libp2p es P2P_PORT + 1000)
# API_PORT (HTTP API principal)
# METRICS_PORT (Prometheus metrics)
# PBFT_WS_PORT (Websocket para PBFT)
# PBOS_WS_PORT (Websocket para PBOS)
# LIBP2P_PORT (P2P_PORT + 1000, para descubrimiento y comunicación directa)
# Nota: P2P_PORT se expone para compatibilidad con chequeos de salud/API en ese puerto si aplica,
# pero el puerto crucial para libp2p es P2P_PORT + 1000.
# Docker EXPOSE no mapea puertos, solo los documenta. El mapeo se hace en `docker run -p` o Kubernetes.
EXPOSE ${P2P_PORT} ${API_PORT} ${METRICS_PORT} ${PBFT_WS_PORT} ${PBOS_WS_PORT}
# EXPOSE 4001 # Ejemplo si P2P_PORT=3001, entonces libp2p escucha en 4001. Se podría añadir dinámicamente en el entrypoint si fuera necesario.

# Volúmenes para datos persistentes
VOLUME ["/app/data", "/app/logs", "/app/config"]

# Health check mejorado (asume que API_PORT es el puerto para health checks)
HEALTHCHECK --interval=30s --timeout=10s --start-period=60s --retries=5 \
    CMD curl -f http://localhost:${API_PORT}/health || exit 1

CMD ["/usr/local/bin/dpos-node"]