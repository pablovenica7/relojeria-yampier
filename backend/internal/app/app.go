// Package app arma la aplicación: compone dependencias de forma explícita
// (Mongo → repositories → services → controllers → router), sin globals ni
// init(), y corre el servidor HTTP con cierre ordenado.
package app

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"relojeria-yampier/internal/auth"
	"relojeria-yampier/internal/config"
	"relojeria-yampier/internal/controller"
	"relojeria-yampier/internal/database"
	"relojeria-yampier/internal/mail"
	"relojeria-yampier/internal/middleware"
	"relojeria-yampier/internal/repository"
	"relojeria-yampier/internal/service"
)

// Services agrupa los casos de uso (expuestos para los tests de integración).
type Services struct {
	Stock         *service.StockService
	Watches       *service.WatchService
	Reservations  *service.ReservationService
	Customers     *service.CustomerService
	AdminAuth     *service.AdminAuthService
	PasswordReset *service.PasswordResetService
	Inquiries     *service.InquiryService
	Dashboard     *service.DashboardService
	Uploads       *service.UploadService
}

// App es la aplicación lista para correr.
type App struct {
	cfg      config.Config
	db       *database.DB
	log      *slog.Logger
	Tokens   *auth.TokenManager
	Services Services
	handler  http.Handler
}

// New compone todas las dependencias. No abre conexiones ni lee el entorno:
// recibe la configuración y la base ya conectadas.
func New(cfg config.Config, db *database.DB, log *slog.Logger) *App {
	repos := struct {
		watches      *repository.WatchRepository
		reservations *repository.ReservationRepository
		customers    *repository.CustomerRepository
		admins       *repository.AdminRepository
		inquiries    *repository.InquiryRepository
		moves        *repository.StockMovementRepository
		resets       *repository.PasswordResetRepository
	}{
		watches:      repository.NewWatchRepository(db.Collection(database.CollWatches)),
		reservations: repository.NewReservationRepository(db.Collection(database.CollReservations)),
		customers:    repository.NewCustomerRepository(db.Collection(database.CollCustomers)),
		admins:       repository.NewAdminRepository(db.Collection(database.CollAdmins)),
		inquiries:    repository.NewInquiryRepository(db.Collection(database.CollInquiries)),
		moves:        repository.NewStockMovementRepository(db.Collection(database.CollStockMovements)),
		resets:       repository.NewPasswordResetRepository(db.Collection(database.CollPasswordResets)),
	}
	tokens := auth.NewTokenManager(cfg.JWTSecret)

	stock := service.NewStockService(repos.watches, repos.moves, log, nil)
	reservations := service.NewReservationService(repos.reservations, repos.watches, repos.customers, stock, log, nil)
	svc := Services{
		Stock:         stock,
		Watches:       service.NewWatchService(repos.watches, repos.reservations, stock, nil),
		Reservations:  reservations,
		Customers:     service.NewCustomerService(repos.customers, tokens, nil),
		AdminAuth:     service.NewAdminAuthService(repos.admins, tokens),
		PasswordReset: service.NewPasswordResetService(repos.customers, repos.resets, mail.New(cfg.Mail, log), cfg.PublicURL, log, nil),
		Inquiries:     service.NewInquiryService(repos.inquiries, repos.watches, nil),
		Dashboard:     service.NewDashboardService(repos.reservations, repos.watches, repos.inquiries, reservations, nil),
		Uploads:       service.NewUploadService(cfg.UploadsDir, nil),
	}

	a := &App{cfg: cfg, db: db, log: log, Tokens: tokens, Services: svc}
	a.handler = a.routes(handlers{
		watches:      controller.NewWatchHandler(svc.Watches),
		reservations: controller.NewReservationHandler(svc.Reservations),
		accounts:     controller.NewAccountHandler(svc.Customers, svc.PasswordReset, svc.AdminAuth, log),
		inquiries:    controller.NewInquiryHandler(svc.Inquiries),
		admin:        controller.NewAdminHandler(svc.Dashboard, svc.Customers, svc.Stock, svc.Uploads),
		auth:         middleware.NewAuth(tokens, svc.AdminAuth, svc.Customers),
	})
	return a
}

// Handler devuelve el router completo (útil para tests con httptest).
func (a *App) Handler() http.Handler { return a.handler }

// Prepare corre las tareas de arranque de la base: backfill aditivo de
// documentos viejos (antes de los índices: completa modelKey/brandKey),
// índices y seeds.
func (a *App) Prepare(ctx context.Context) {
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	a.db.BackfillLegacyData(ctx, a.log)
	a.db.EnsureIndexes(ctx, a.log)
	a.db.SeedAdmin(ctx, a.cfg.AdminEmail, a.cfg.AdminPassword, a.log)
	if a.cfg.SeedDemoData {
		a.db.SeedDemoWatches(ctx, a.log)
	}
}

// Run levanta el servidor HTTP y el vencimiento periódico de reservas, y los
// detiene ordenadamente ante SIGINT/SIGTERM (los requests en curso terminan
// dentro de HTTP.ShutdownTimeout).
func (a *App) Run(ctx context.Context) error {
	ctx, stop := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go a.Services.Reservations.RunExpirationSweeper(ctx, a.cfg.ReservationSweepInterval)

	srv := &http.Server{
		Addr:              ":" + a.cfg.Port,
		Handler:           a.handler,
		ReadHeaderTimeout: a.cfg.HTTP.ReadHeaderTimeout,
		ReadTimeout:       a.cfg.HTTP.ReadTimeout,
		WriteTimeout:      a.cfg.HTTP.WriteTimeout,
		IdleTimeout:       a.cfg.HTTP.IdleTimeout,
	}
	errCh := make(chan error, 1)
	go func() {
		a.log.Info("API escuchando", "port", a.cfg.Port)
		errCh <- srv.ListenAndServe()
	}()

	select {
	case err := <-errCh:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-ctx.Done():
		a.log.Info("señal de apagado recibida: cerrando el servidor")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), a.cfg.HTTP.ShutdownTimeout)
		defer cancel()
		return srv.Shutdown(shutdownCtx)
	}
}
