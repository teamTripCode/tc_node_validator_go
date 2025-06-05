# Build stage
FROM golang:1.22-alpine as builder

# Instalar dependencias necesarias
RUN apk add --no-cache git ca-certificates tzdata

# Crear usuario no-root
RUN adduser -D -s /bin/sh -u 1001 appuser

WORKDIR /app

# Copiar archivos de dependencias primero para mejor cache
COPY go.mod go.sum ./
RUN go mod download

# Copiar código fuente
COPY . .

# Build con optimizaciones
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags='-w -s -extldflags "-static"' \
    -a -installsuffix cgo \
    -o /dpos-node .

# Final stage
FROM gcr.io/distroless/static-debian12

# Copiar usuario desde builder
COPY --from=builder /etc/passwd /etc/passwd
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo

# Copiar binario
COPY --from=builder /dpos-node /dpos-node

# Cambiar a usuario no-root
USER appuser

# Exponer puerto
EXPOSE 3001

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD ["/dpos-node", "--health-check"] || exit 1

# Comando por defecto
CMD ["/dpos-node"]