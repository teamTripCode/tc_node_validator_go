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

# Final stage - Cambiar a imagen base que permita escritura
FROM alpine:3.19

# Instalar dependencias mínimas
RUN apk add --no-cache ca-certificates tzdata

# Crear usuario no-root con el mismo UID que el StatefulSet
RUN adduser -D -s /bin/sh -u 1000 appuser

# Crear directorios necesarios con permisos correctos
RUN mkdir -p /data/blockchain /data/tx_chain /var/log/validator-node /tmp && \
    chown -R appuser:appuser /data /var/log/validator-node /tmp && \
    chmod -R 755 /data /var/log/validator-node && \
    chmod -R 1777 /tmp

# Copiar binario
COPY --from=builder /dpos-node /usr/local/bin/dpos-node

# Cambiar a usuario no-root
USER appuser

WORKDIR /data

EXPOSE 3001 3002 9090

HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD ["/usr/local/bin/dpos-node", "--health-check"] || exit 1

CMD ["/usr/local/bin/dpos-node"]