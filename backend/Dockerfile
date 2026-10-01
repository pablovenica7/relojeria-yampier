# --- Etapa de compilación ---
FROM golang:1.22-alpine AS build
WORKDIR /app
COPY go.mod go.sum* ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /app/server .

# --- Imagen final, liviana ---
FROM alpine:3.20
WORKDIR /app
RUN apk add --no-cache ca-certificates
COPY --from=build /app/server ./server
# Carpeta donde se guardan las imágenes subidas desde el panel Admin.
# En docker-compose se monta como volumen para que persista entre despliegues.
RUN mkdir -p /app/uploads
ENV UPLOADS_DIR=/app/uploads
EXPOSE 8080
CMD ["./server"]
