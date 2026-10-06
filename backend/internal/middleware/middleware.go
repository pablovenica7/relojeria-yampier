// Package middleware tiene todo lo que envuelve a los handlers: CORS,
// request ID, logging, recovery, timeout por request, rate limiting y
// autenticación/autorización (Admin y cliente).
package middleware

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"net/http"
	"runtime/debug"
	"time"

	"relojeria-yampier/internal/httpx"
)

// Chain aplica middlewares en orden: Chain(h, a, b) == a(b(h)).
func Chain(h http.Handler, mws ...func(http.Handler) http.Handler) http.Handler {
	for i := len(mws) - 1; i >= 0; i-- {
		h = mws[i](h)
	}
	return h
}

// CORS permite un único origen explícito, nunca "*": el panel envía un token
// Bearer y solo el frontend configurado puede usarlo desde el navegador.
func CORS(allowedOrigin string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Access-Control-Allow-Origin", allowedOrigin)
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
			w.Header().Set("Vary", "Origin")
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func newRequestID() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return "unknown"
	}
	return hex.EncodeToString(b)
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (s *statusRecorder) WriteHeader(code int) {
	s.status = code
	s.ResponseWriter.WriteHeader(code)
}

// RequestLog asigna un request ID (generado SIEMPRE en el servidor; se
// devuelve en X-Request-ID) y registra cada request en una línea:
//
//	msg=request method=GET path=/api/watches status=200 duration_ms=23 ip=... request_id=...
//
// Solo método, ruta (sin query string: puede tener búsquedas o tokens),
// status, duración e IP. Nunca headers (Authorization lleva el JWT) ni
// cuerpos (contraseñas, documentos, datos fiscales).
func RequestLog(log *slog.Logger, proxies *TrustedProxies) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			id := newRequestID()
			w.Header().Set("X-Request-ID", id)
			rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
			r = r.WithContext(httpx.WithRequestID(r.Context(), id))
			next.ServeHTTP(rec, r)
			log.Info("request", "method", r.Method, "path", r.URL.Path, "status", rec.status,
				"duration_ms", time.Since(start).Milliseconds(), "ip", proxies.ClientIP(r), "request_id", id)
		})
	}
}

// Recover convierte un panic en un 500 seguro (con el stack solo en el log),
// en vez de cortar la conexión sin respuesta.
func Recover(log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if v := recover(); v != nil {
					if v == http.ErrAbortHandler {
						panic(v)
					}
					log.Error("panic en handler", "request_id", httpx.RequestID(r.Context()),
						"path", r.URL.Path, "panic", v, "stack", string(debug.Stack()))
					httpx.WriteMessage(w, http.StatusInternalServerError, "", "Error interno del servidor")
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}

// Timeout fija un deadline al contexto del request. Se propaga por
// controller → service → repository → MongoDB, así una consulta lenta no
// queda colgada indefinidamente.
func Timeout(d time.Duration) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx, cancel := context.WithTimeout(r.Context(), d)
			defer cancel()
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
