package controller

import (
	"context"
	"net/http"
	"time"

	"relojeria-yampier/internal/httpx"
)

// Pinger verifica la base (implementado por database.DB).
type Pinger interface {
	Ping(ctx context.Context) error
}

// Health: GET /api/health — sin URI, versiones ni detalles internos.
func Health(db Pinger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if err := db.Ping(ctx); err != nil {
			httpx.WriteJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "degraded", "database": "unavailable"})
			return
		}
		httpx.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok", "database": "ok"})
	}
}

// NotFound responde 404 JSON para rutas /api/ inexistentes.
func NotFound(w http.ResponseWriter, _ *http.Request) {
	httpx.WriteMessage(w, http.StatusNotFound, "", "Recurso no encontrado")
}
