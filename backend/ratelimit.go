package main

import (
	"log"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

// rateLimiter es un limitador simple de ventana fija, en memoria, por IP.
//
// Limitación conocida y aceptada: si el backend corre en más de una réplica
// (varias instancias detrás de un balanceador), cada una lleva su propio
// contador, así que el límite real es "maxRequests × cantidad de réplicas".
// Para esta aplicación (una sola instancia de backend) es suficiente y evita
// depender de Redis u otro almacenamiento compartido solo para esto.
type rateLimiter struct {
	mu       sync.Mutex
	hits     map[string][]time.Time
	window   time.Duration
	max      int
	lastSwap time.Time
	proxies  *trustedProxies
}

func newRateLimiter(max int, window time.Duration, proxies *trustedProxies) *rateLimiter {
	return &rateLimiter{
		hits:     make(map[string][]time.Time),
		window:   window,
		max:      max,
		lastSwap: time.Now(),
		proxies:  proxies,
	}
}

// trustedProxies es la lista de proxies propios (TRUSTED_PROXIES) cuyo
// header X-Forwarded-For se acepta. Por defecto está vacía: sin proxy
// configurado, X-Forwarded-For se ignora por completo, porque cualquier
// cliente puede enviarlo falso para evadir el rate limiting.
type trustedProxies struct {
	nets []*net.IPNet
}

// parseTrustedProxies acepta IPs o rangos CIDR separados por coma, por
// ejemplo "127.0.0.1,172.16.0.0/12". Los valores inválidos se ignoran con aviso.
func parseTrustedProxies(spec string) *trustedProxies {
	tp := &trustedProxies{}
	for _, part := range strings.Split(spec, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if !strings.Contains(part, "/") {
			if ip := net.ParseIP(part); ip != nil {
				if ip.To4() != nil {
					part += "/32"
				} else {
					part += "/128"
				}
			}
		}
		_, n, err := net.ParseCIDR(part)
		if err != nil {
			log.Printf("TRUSTED_PROXIES: valor inválido ignorado: %q", part)
			continue
		}
		tp.nets = append(tp.nets, n)
	}
	return tp
}

func (tp *trustedProxies) trusts(ip net.IP) bool {
	if tp == nil || ip == nil {
		return false
	}
	for _, n := range tp.nets {
		if n.Contains(ip) {
			return true
		}
	}
	return false
}

// clientIP devuelve la IP real del cliente. Si la conexión viene de un proxy
// confiable, recorre X-Forwarded-For de derecha a izquierda (cada proxy
// agrega la IP que vio al final) y devuelve la primera IP que NO es un proxy
// confiable. Las entradas más a la izquierda las puede inventar el cliente,
// por eso nunca se toma la primera ciegamente.
func (tp *trustedProxies) clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	if !tp.trusts(net.ParseIP(host)) {
		return host
	}
	hops := strings.Split(strings.Join(r.Header.Values("X-Forwarded-For"), ","), ",")
	for i := len(hops) - 1; i >= 0; i-- {
		hop := strings.TrimSpace(hops[i])
		if hop == "" {
			continue
		}
		ip := net.ParseIP(hop)
		if ip == nil {
			return host // header malformado: se usa la IP del proxy
		}
		if !tp.trusts(ip) {
			return ip.String()
		}
	}
	return host
}

func (rl *rateLimiter) allow(key string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()

	// Housekeeping ocasional para que el mapa no crezca sin límite.
	if now.Sub(rl.lastSwap) > 10*time.Minute {
		rl.hits = make(map[string][]time.Time)
		rl.lastSwap = now
	}

	cutoff := now.Add(-rl.window)
	var kept []time.Time
	for _, t := range rl.hits[key] {
		if t.After(cutoff) {
			kept = append(kept, t)
		}
	}
	if len(kept) >= rl.max {
		rl.hits[key] = kept
		return false
	}
	kept = append(kept, now)
	rl.hits[key] = kept
	return true
}

func (rl *rateLimiter) middleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !rl.allow(rl.proxies.clientIP(r)) {
			writeJSON(w, http.StatusTooManyRequests, map[string]string{
				"error": "Demasiados intentos. Esperá un momento y volvé a intentarlo.",
			})
			return
		}
		next(w, r)
	}
}
