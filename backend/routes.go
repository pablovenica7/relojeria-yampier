package main

import (
	"net/http"
	"time"
)

// newRouter arma todas las rutas de la API. Está separado de main() para que
// los tests puedan ejercitar la API HTTP real (middlewares de auth incluidos).
func newRouter(proxies *trustedProxies) http.Handler {
	loginLimiter := newRateLimiter(5, time.Minute, proxies)           // login Admin: 5/min por IP
	customerLoginLimiter := newRateLimiter(5, time.Minute, proxies)   // login clientes: 5/min por IP
	registerLimiter := newRateLimiter(5, 10*time.Minute, proxies)     // registros: 5 cada 10 min por IP
	inquiryLimiter := newRateLimiter(10, time.Minute, proxies)        // consultas: 10/min por IP
	reservationLimiter := newRateLimiter(10, 10*time.Minute, proxies) // solicitudes de reserva
	passwordLimiter := newRateLimiter(5, 10*time.Minute, proxies)     // cambios de contraseña
	forgotLimiter := newRateLimiter(3, 15*time.Minute, proxies)       // pedidos de recuperación
	resetLimiter := newRateLimiter(10, 15*time.Minute, proxies)       // intentos de restablecer

	mux := http.NewServeMux()

	// Rutas públicas
	mux.HandleFunc("GET /api/health", healthHandler)
	mux.HandleFunc("GET /api/watches", getWatches)
	mux.HandleFunc("GET /api/watches/{id}", getWatch)
	mux.HandleFunc("POST /api/inquiries", inquiryLimiter.middleware(postInquiry))
	mux.HandleFunc("POST /api/admin/login", loginLimiter.middleware(postLogin))

	// Clientes: cuenta propia (rol "customer"; nunca da acceso al Admin)
	mux.HandleFunc("POST /api/auth/register", registerLimiter.middleware(postRegister))
	mux.HandleFunc("POST /api/auth/login", customerLoginLimiter.middleware(postCustomerLogin))
	mux.HandleFunc("POST /api/auth/password/forgot", forgotLimiter.middleware(postForgotPassword))
	mux.HandleFunc("POST /api/auth/password/reset", resetLimiter.middleware(postResetPassword))
	mux.HandleFunc("GET /api/auth/me", requireCustomer(getMe))
	mux.HandleFunc("PUT /api/auth/me", requireCustomer(putMe))
	mux.HandleFunc("POST /api/auth/me/password", passwordLimiter.middleware(requireCustomer(postChangePassword)))

	// Clientes: sus reservas (siempre filtradas por el dueño del token)
	mux.HandleFunc("POST /api/reservations", reservationLimiter.middleware(requireCustomer(postCustomerReservation)))
	mux.HandleFunc("GET /api/reservations", requireCustomer(getCustomerReservations))
	mux.HandleFunc("GET /api/reservations/{id}", requireCustomer(getCustomerReservation))
	mux.HandleFunc("POST /api/reservations/{id}/cancel", requireCustomer(postCustomerCancel))

	// Rutas protegidas (panel Admin: rol "admin")
	mux.HandleFunc("GET /api/admin/dashboard", requireAuth(getDashboard))
	mux.HandleFunc("GET /api/admin/stock-movements", requireAuth(listStockMovements))
	mux.HandleFunc("GET /api/admin/customers", requireAuth(listCustomers))
	mux.HandleFunc("PATCH /api/admin/customers/{id}/deactivate", requireAuth(deactivateCustomer))
	mux.HandleFunc("PATCH /api/admin/customers/{id}/reactivate", requireAuth(reactivateCustomer))
	mux.HandleFunc("GET /api/admin/watches", requireAuth(adminListWatches))
	mux.HandleFunc("GET /api/admin/watches/{id}", requireAuth(adminGetWatch))
	mux.HandleFunc("POST /api/admin/watches", requireAuth(createWatch))
	mux.HandleFunc("PUT /api/admin/watches/{id}", requireAuth(updateWatch))
	mux.HandleFunc("PATCH /api/admin/watches/{id}/stock", requireAuth(patchWatchStock))
	mux.HandleFunc("PATCH /api/admin/watches/{id}/archive", requireAuth(archiveWatch))
	mux.HandleFunc("PATCH /api/admin/watches/{id}/restore", requireAuth(restoreWatch))
	mux.HandleFunc("DELETE /api/admin/watches/{id}", requireAuth(deleteWatch)) // excepcional: solo archivados sin reservas

	mux.HandleFunc("GET /api/admin/inquiries", requireAuth(listInquiries))
	mux.HandleFunc("PATCH /api/admin/inquiries/{id}", requireAuth(patchInquiry))
	mux.HandleFunc("DELETE /api/admin/inquiries/{id}", requireAuth(deleteInquiry))

	mux.HandleFunc("GET /api/admin/reservations", requireAuth(listReservations))
	mux.HandleFunc("POST /api/admin/reservations", requireAuth(postReservation))
	mux.HandleFunc("PATCH /api/admin/reservations/{id}", requireAuth(patchReservation))
	mux.HandleFunc("PATCH /api/admin/reservations/{id}/status", requireAuth(patchReservationStatus))

	mux.HandleFunc("POST /api/admin/uploads", requireAuth(postUpload))

	// Cualquier otra ruta /api/ responde 404 en JSON (no texto plano).
	mux.HandleFunc("/api/", apiNotFound)

	// Imágenes subidas, servidas de forma pública
	mux.Handle("GET /uploads/", http.StripPrefix("/uploads/", http.FileServer(http.Dir(uploadsDir()))))

	return logRequests(proxies, cors(mux))
}
