# Dockerfile simplificado para resolver errores de build
FROM golang:1.23.5-alpine AS builder

# Instalar herramientas necesarias
RUN apk add --no-cache git ca-certificates tzdata wget curl

# Configurar directorio de trabajo
WORKDIR /app

# Copiar archivos de módulos Go
COPY go.mod go.sum ./

# Descargar dependencias
RUN go mod download && go mod tidy

# Copiar código fuente
COPY . .

# Compilar la aplicación
RUN CGO_ENABLED=0 GOOS=linux go build -o main cmd/api/main.go

# Stage final
FROM alpine:3.19

# Instalar herramientas básicas
RUN apk --no-cache add ca-certificates wget curl tzdata && \
    update-ca-certificates

# Crear usuario no-root
RUN addgroup -g 1001 -S appgroup && \
    adduser -u 1001 -S appuser -G appgroup

# Configurar directorio de trabajo
WORKDIR /app

# Copiar binario desde builder
COPY --from=builder /app/main .

# Copiar configuración
COPY .env.example .env

# Cambiar propietario
RUN chown -R appuser:appgroup /app

# Cambiar a usuario no-root
USER appuser

# Exponer puerto
EXPOSE 8081

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=60s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8081/api/v1/health || exit 1

# Comando por defecto
CMD ["./main"]