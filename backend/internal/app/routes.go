package app

import (
	"net/http"
	"time"

	"relojeria-yampier/internal/controller"
	"relojeria-yampier/internal/middleware"
)

type handlers struct {
	watches      *controller.WatchHandler
	reservations *controller.ReservationHandler
	accounts     *controller.AccountHandler
	inquiries    *controller.InquiryHandler
	admin        *controller.AdminHandler
	auth         *middleware.Auth
}

// requestTimeout es el deadline de cada request (se propaga hasta Mongo).
const requestTimeout = 15 * time.Second

// routes arma el router con las rutas agrupadas por dominio. Las URLs,
// métodos y límites son los mismos que consume el frontend.
func (a *App) routes(h handlers) http.Handler {
	proxies := middleware.ParseTrustedProxies(a.cfg.TrustedProxies, a.log)
	limit := func(max int, window time.Duration) *middleware.RateLimiter {
		return middleware.NewRateLimiter(max, window, proxies)
	}
	admin, customer := h.auth.RequireAdmin, h.auth.RequireCustomer

	mux := http.NewServeMux()

	// /api/health
	mux.HandleFunc("GET /api/health", controller.Health(a.db))

	// /api/watches (catálogo público)
	mux.HandleFunc("GET /api/watches", h.watches.List)
	mux.HandleFunc("GET /api/watches/{id}", h.watches.Get)

	// /api/inquiries (consultas públicas)
	mux.HandleFunc("POST /api/inquiries", limit(10, time.Minute).Limit(h.inquiries.Create))

	// /api/auth (cuentas de clientes; rol "customer", nunca da acceso al Admin)
	mux.HandleFunc("POST /api/auth/register", limit(5, 10*time.Minute).Limit(h.accounts.Register))
	mux.HandleFunc("POST /api/auth/login", limit(5, time.Minute).Limit(h.accounts.Login))
	mux.HandleFunc("POST /api/auth/password/forgot", limit(3, 15*time.Minute).Limit(h.accounts.ForgotPassword))
	mux.HandleFunc("POST /api/auth/password/reset", limit(10, 15*time.Minute).Limit(h.accounts.ResetPassword))
	mux.HandleFunc("GET /api/auth/me", customer(h.accounts.Me))
	mux.HandleFunc("PUT /api/auth/me", customer(h.accounts.UpdateMe))
	mux.HandleFunc("POST /api/auth/me/password", limit(5, 10*time.Minute).Limit(customer(h.accounts.ChangePassword)))

	// /api/reservations (reservas del cliente; siempre filtradas por el dueño del token)
	mux.HandleFunc("POST /api/reservations", limit(10, 10*time.Minute).Limit(customer(h.reservations.CustomerCreate)))
	mux.HandleFunc("GET /api/reservations", customer(h.reservations.CustomerList))
	mux.HandleFunc("GET /api/reservations/{id}", customer(h.reservations.CustomerGet))
	mux.HandleFunc("POST /api/reservations/{id}/cancel", customer(h.reservations.CustomerCancel))

	// /api/admin (panel; rol "admin")
	mux.HandleFunc("POST /api/admin/login", limit(5, time.Minute).Limit(h.accounts.AdminLogin))
	mux.HandleFunc("GET /api/admin/dashboard", admin(h.admin.Dashboard))
	mux.HandleFunc("POST /api/admin/uploads", admin(h.admin.Upload))
	// relojes y stock
	mux.HandleFunc("GET /api/admin/watches", admin(h.watches.AdminList))
	mux.HandleFunc("GET /api/admin/watches/{id}", admin(h.watches.AdminGet))
	mux.HandleFunc("POST /api/admin/watches", admin(h.watches.Create))
	mux.HandleFunc("PUT /api/admin/watches/{id}", admin(h.watches.Update))
	mux.HandleFunc("PATCH /api/admin/watches/{id}/stock", admin(h.watches.AdjustStock))
	mux.HandleFunc("PATCH /api/admin/watches/{id}/archive", admin(h.watches.Archive))
	mux.HandleFunc("PATCH /api/admin/watches/{id}/restore", admin(h.watches.Restore))
	mux.HandleFunc("DELETE /api/admin/watches/{id}", admin(h.watches.Delete)) // excepcional
	mux.HandleFunc("GET /api/admin/stock-movements", admin(h.admin.StockMovements))
	// reservas
	mux.HandleFunc("GET /api/admin/reservations", admin(h.reservations.AdminList))
	mux.HandleFunc("POST /api/admin/reservations", admin(h.reservations.AdminCreate))
	mux.HandleFunc("PATCH /api/admin/reservations/{id}", admin(h.reservations.AdminUpdateDetails))
	mux.HandleFunc("PATCH /api/admin/reservations/{id}/status", admin(h.reservations.AdminChangeStatus))
	// consultas
	mux.HandleFunc("GET /api/admin/inquiries", admin(h.inquiries.List))
	mux.HandleFunc("PATCH /api/admin/inquiries/{id}", admin(h.inquiries.Update))
	mux.HandleFunc("DELETE /api/admin/inquiries/{id}", admin(h.inquiries.Delete))
	// /api/customers (gestión de cuentas desde el panel)
	mux.HandleFunc("GET /api/admin/customers", admin(h.admin.Customers))
	mux.HandleFunc("PATCH /api/admin/customers/{id}/deactivate", admin(h.admin.DeactivateCustomer))
	mux.HandleFunc("PATCH /api/admin/customers/{id}/reactivate", admin(h.admin.ReactivateCustomer))

	// Cualquier otra ruta /api/ responde 404 en JSON (no texto plano).
	mux.HandleFunc("/api/", controller.NotFound)

	// Imágenes subidas, servidas de forma pública.
	mux.Handle("GET /uploads/", http.StripPrefix("/uploads/", http.FileServer(http.Dir(a.Services.Uploads.Dir()))))

	return middleware.Chain(mux,
		middleware.RequestLog(a.log, proxies),
		middleware.Recover(a.log),
		middleware.CORS(a.cfg.AllowedOrigin),
		middleware.Timeout(requestTimeout),
	)
}
