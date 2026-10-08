# --- Etapa de compilación ---
FROM golang:1.22-alpine AS build
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /app/server ./cmd/api

# --- Imagen final, liviana: solo el binario, sin código fuente ni secretos ---
FROM alpine:3.20
WORKDIR /app
RUN apk add --no-cache ca-certificates tzdata \
    && adduser -D -H -u 10001 app \
    && mkdir -p /app/uploads && chown app /app/uploads
COPY --from=build /app/server ./server
# Las imágenes nuevas se guardan en MongoDB (GridFS). /app/uploads solo se lee
# al arrancar para migrar imágenes de versiones anteriores (volumen de
# docker-compose); en el hosting queda vacía.
ENV UPLOADS_DIR=/app/uploads
USER app
# El hosting define PORT (Render usa 10000); 8080 es el valor local.
EXPOSE 8080
CMD ["./server"]
