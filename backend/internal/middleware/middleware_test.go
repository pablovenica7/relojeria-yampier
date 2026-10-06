package middleware

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestClientIPIgnoresForwardedForWithoutTrustedProxy(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	r.RemoteAddr = "203.0.113.7:5555"
	r.Header.Set("X-Forwarded-For", "1.1.1.1")
	if got := ParseTrustedProxies("", nil).ClientIP(r); got != "203.0.113.7" {
		t.Errorf("sin proxy confiable debe usarse RemoteAddr, obtuvo %q", got)
	}
}

func TestClientIPBehindTrustedProxy(t *testing.T) {
	tp := ParseTrustedProxies("10.0.0.0/8, 127.0.0.1, basura", nil)
	r := httptest.NewRequest("GET", "/", nil)
	r.RemoteAddr = "10.0.0.5:4444"
	r.Header.Set("X-Forwarded-For", "9.9.9.9, 198.51.100.20")
	if got := tp.ClientIP(r); got != "198.51.100.20" {
		t.Errorf("debería tomarse la IP agregada por el proxy, obtuvo %q", got)
	}
	r.Header.Set("X-Forwarded-For", "198.51.100.20, 10.0.0.9")
	if got := tp.ClientIP(r); got != "198.51.100.20" {
		t.Errorf("debería saltear proxies confiables, obtuvo %q", got)
	}
	r.Header.Set("X-Forwarded-For", "no-es-ip")
	if got := tp.ClientIP(r); got != "10.0.0.5" {
		t.Errorf("con header malformado debería usarse RemoteAddr, obtuvo %q", got)
	}
}

func TestRateLimiterBlocksAfterMax(t *testing.T) {
	rl := NewRateLimiter(2, time.Minute, nil)
	if !rl.Allow("a") || !rl.Allow("a") {
		t.Fatal("los dos primeros intentos deberían pasar")
	}
	if rl.Allow("a") {
		t.Error("el tercer intento debería bloquearse")
	}
	if !rl.Allow("b") {
		t.Error("otra IP no debería verse afectada")
	}
}

func TestRecoverTurnsPanicInto500(t *testing.T) {
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	h := Recover(log)(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { panic("boom") }))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/api/x", nil))
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("un panic debe responder 500, respondió %d", rec.Code)
	}
}

func TestRequestLogSetsRequestID(t *testing.T) {
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	h := RequestLog(log, nil)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(204) }))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/api/health?token=secreto", nil))
	if rec.Header().Get("X-Request-ID") == "" {
		t.Error("cada respuesta debe llevar X-Request-ID")
	}
}
