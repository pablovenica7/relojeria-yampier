package middleware

import (
	"log/slog"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"relojeria-yampier/internal/httpx"
)

// RateLimiter es un limitador de ventana deslizante, en memoria, por IP.
//
// Limitación conocida y aceptada: con varias réplicas del backend, cada una
// lleva su propio contador. Para una sola instancia es suficiente y evita
// depender de Redis solo para esto.
type RateLimiter struct {
	mu       sync.Mutex
	hits     map[string][]time.Time
	window   time.Duration
	max      int
	lastSwap time.Time
	proxies  *TrustedProxies
}

func NewRateLimiter(max int, window time.Duration, proxies *TrustedProxies) *RateLimiter {
	return &RateLimiter{hits: map[string][]time.Time{}, window: window, max: max, lastSwap: time.Now(), proxies: proxies}
}

// Allow registra un intento de key y dice si está dentro del límite.
func (rl *RateLimiter) Allow(key string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	now := time.Now()
	if now.Sub(rl.lastSwap) > 10*time.Minute { // limpieza ocasional
		rl.hits = map[string][]time.Time{}
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
	rl.hits[key] = append(kept, now)
	return true
}

// Limit envuelve un handler con el límite por IP.
func (rl *RateLimiter) Limit(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !rl.Allow(rl.proxies.ClientIP(r)) {
			httpx.WriteMessage(w, http.StatusTooManyRequests, "", "Demasiados intentos. Esperá un momento y volvé a intentarlo.")
			return
		}
		next(w, r)
	}
}

// TrustedProxies es la lista de proxies propios (TRUSTED_PROXIES) cuyo
// X-Forwarded-For se acepta. Vacía por defecto: sin proxy configurado,
// X-Forwarded-For se ignora (cualquier cliente puede falsearlo).
type TrustedProxies struct{ nets []*net.IPNet }

// ParseTrustedProxies acepta IPs o CIDR separados por coma. Los valores
// inválidos se ignoran con aviso en el log.
func ParseTrustedProxies(spec string, log *slog.Logger) *TrustedProxies {
	tp := &TrustedProxies{}
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
			if log != nil {
				log.Warn("TRUSTED_PROXIES: valor inválido ignorado", "value", part)
			}
			continue
		}
		tp.nets = append(tp.nets, n)
	}
	return tp
}

func (tp *TrustedProxies) trusts(ip net.IP) bool {
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

// ClientIP devuelve la IP real del cliente. Si la conexión viene de un proxy
// confiable, recorre X-Forwarded-For de derecha a izquierda y devuelve la
// primera IP que NO es un proxy confiable (las de la izquierda las puede
// inventar el cliente).
func (tp *TrustedProxies) ClientIP(r *http.Request) string {
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
