package main

import (
	"net/http/httptest"
	"testing"
)

func TestClientIPIgnoresForwardedForWithoutTrustedProxy(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	r.RemoteAddr = "203.0.113.7:5555"
	r.Header.Set("X-Forwarded-For", "1.1.1.1")
	if got := parseTrustedProxies("").clientIP(r); got != "203.0.113.7" {
		t.Errorf("sin proxy confiable debe usarse RemoteAddr, obtuvo %q", got)
	}
}

func TestClientIPBehindTrustedProxy(t *testing.T) {
	tp := parseTrustedProxies("10.0.0.0/8, 127.0.0.1, basura")
	r := httptest.NewRequest("GET", "/", nil)
	r.RemoteAddr = "10.0.0.5:4444"
	// El cliente intenta falsear su IP agregando 9.9.9.9; Nginx agrega la real al final.
	r.Header.Set("X-Forwarded-For", "9.9.9.9, 198.51.100.20")
	if got := tp.clientIP(r); got != "198.51.100.20" {
		t.Errorf("debería tomarse la IP agregada por el proxy, obtuvo %q", got)
	}

	// Cadena de proxies propios: se saltean todos los confiables.
	r.Header.Set("X-Forwarded-For", "198.51.100.20, 10.0.0.9")
	if got := tp.clientIP(r); got != "198.51.100.20" {
		t.Errorf("debería saltear proxies confiables, obtuvo %q", got)
	}

	// Header malformado: se usa la IP del proxy (conservador).
	r.Header.Set("X-Forwarded-For", "no-es-ip")
	if got := tp.clientIP(r); got != "10.0.0.5" {
		t.Errorf("con header malformado debería usarse RemoteAddr, obtuvo %q", got)
	}
}

func TestRateLimiterBlocksAfterMax(t *testing.T) {
	rl := newRateLimiter(2, 60e9, nil)
	if !rl.allow("a") || !rl.allow("a") {
		t.Fatal("los dos primeros intentos deberían pasar")
	}
	if rl.allow("a") {
		t.Error("el tercer intento debería bloquearse")
	}
	if !rl.allow("b") {
		t.Error("otra IP no debería verse afectada")
	}
}
