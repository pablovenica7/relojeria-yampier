package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log"
	"net/http"
	"time"
)

type ctxKey int

const requestIDKey ctxKey = iota

func newRequestID() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return "unknown"
	}
	return hex.EncodeToString(b)
}

func requestIDFrom(r *http.Request) string {
	if r == nil {
		return ""
	}
	if id, ok := r.Context().Value(requestIDKey).(string); ok {
		return id
	}
	return ""
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (s *statusRecorder) WriteHeader(code int) {
	s.status = code
	s.ResponseWriter.WriteHeader(code)
}

// logRequests registra cada request en una línea:
//
//	GET /api/watches 200 23ms ip=1.2.3.4 request_id=9f2c...
//
// Solo se loguean método, ruta (sin query string: puede tener datos de
// búsqueda), status, duración, IP y request ID. Nunca headers (Authorization
// lleva el JWT) ni cuerpos (pueden tener contraseñas o datos personales).
// El request ID se genera siempre en el servidor (no se acepta uno enviado
// por el cliente) y se devuelve en el header X-Request-ID para soporte.
func logRequests(proxies *trustedProxies, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		id := newRequestID()
		w.Header().Set("X-Request-ID", id)
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		r = r.WithContext(context.WithValue(r.Context(), requestIDKey, id))
		next.ServeHTTP(rec, r)
		log.Printf("%s %s %d %dms ip=%s request_id=%s",
			r.Method, r.URL.Path, rec.status, time.Since(start).Milliseconds(), proxies.clientIP(r), id)
	})
}
