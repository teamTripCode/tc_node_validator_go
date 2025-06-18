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
RUN mkdir -p /app/data/blockchain \
             /app/data/tx_chain \
             /app/logs \
             /app/tmp \
             /app/config \
             /etc/tripcodechain/mcp && \
    chown -R appuser:appuser /app /etc/tripcodechain && \
    chmod -R 755 /app /etc/tripcodechain && \
    chmod -R 777 /app/tmp && \
    # Crear enlaces simbólicos para compatibilidad
    ln -s /app/data /data && \
    ln -s /app/logs /var/log/validator-node

# Copiar binario
COPY --from=builder /dpos-node /usr/local/bin/dpos-node
RUN chmod +x /usr/local/bin/dpos-node

# Cambiar a usuario no-root
USER appuser

# Establecer directorio de trabajo
WORKDIR /app

# Variables de entorno por defecto
ENV DATA_DIR=/app/data \
    LOG_DIR=/app/logs \
    CONFIG_DIR=/app/config \
    P2P_PORT=3001 \
    API_PORT=3002 \
    MCP_PORT=3003 \
    METRICS_PORT=9090 \
    MCP_METRICS_PORT=9091

EXPOSE 3001 3002 3003 9090 9091

# Health check mejorado
HEALTHCHECK --interval=30s --timeout=10s --start-period=60s --retries=5 \
    CMD curl -f http://localhost:3002/health || exit 1

CMD ["/usr/local/bin/dpos-node"]