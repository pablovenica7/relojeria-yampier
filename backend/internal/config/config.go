// Package config carga TODA la configuración del backend una sola vez al
// arrancar, desde variables de entorno. Ningún otro paquete lee os.Getenv:
// reciben la configuración ya validada por parámetro.
package config

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"
)

// Config es la configuración completa del proceso.
type Config struct {
	AppEnv string // "development" | "production" | vacío
	Port   string

	MongoURI string
	MongoDB  string

	JWTSecret     string
	AdminEmail    string
	AdminPassword string

	AllowedOrigin  string
	TrustedProxies string // IPs/CIDR separados por coma (ver middleware.ParseTrustedProxies)
	UploadsDir     string
	PublicURL      string // URL del sitio para enlaces en emails

	SeedDemoData             bool
	ReservationSweepInterval time.Duration

	Mail MailConfig
	HTTP HTTPConfig
}

// MailConfig configura el envío de emails (ver internal/mail).
type MailConfig struct {
	Driver   string // "smtp" | "log" | vacío
	Host     string
	Port     string
	Username string
	Password string
	From     string
}

// HTTPConfig son los timeouts del servidor HTTP.
type HTTPConfig struct {
	ReadHeaderTimeout time.Duration
	ReadTimeout       time.Duration
	WriteTimeout      time.Duration
	IdleTimeout       time.Duration
	ShutdownTimeout   time.Duration
}

// IsProduction indica si el proceso corre en producción.
func (c Config) IsProduction() bool { return strings.EqualFold(c.AppEnv, "production") }

// Warning es un aviso de configuración no fatal (se registra al arrancar).
type Warning string

// Load lee y valida la configuración. Falla si falta algo crítico (por
// ejemplo JWT_SECRET: no hay clave por defecto, a propósito).
func Load() (Config, []Warning, error) {
	return load(os.Getenv)
}

// load recibe la función de lectura para poder testearla sin tocar el entorno.
func load(getenv func(string) string) (Config, []Warning, error) {
	get := func(k, def string) string {
		if v := strings.TrimSpace(getenv(k)); v != "" {
			return v
		}
		return def
	}
	var warnings []Warning

	cfg := Config{
		AppEnv:         get("APP_ENV", ""),
		Port:           get("PORT", "8080"),
		MongoURI:       get("MONGO_URI", "mongodb://localhost:27017"),
		MongoDB:        get("MONGO_DB", "relojeria_yampier"),
		JWTSecret:      strings.TrimSpace(getenv("JWT_SECRET")),
		AdminEmail:     strings.ToLower(get("ADMIN_EMAIL", "")),
		AdminPassword:  getenv("ADMIN_PASSWORD"), // sin TrimSpace: es una contraseña
		AllowedOrigin:  get("ALLOWED_ORIGIN", "http://localhost:5173"),
		TrustedProxies: get("TRUSTED_PROXIES", ""),
		UploadsDir:     get("UPLOADS_DIR", "./uploads"),
		PublicURL:      strings.TrimRight(get("APP_PUBLIC_URL", "http://localhost:5173"), "/"),
		SeedDemoData:   strings.EqualFold(get("SEED_DEMO_DATA", "false"), "true"),
		Mail: MailConfig{
			Driver:   strings.ToLower(get("MAIL_DRIVER", "")),
			Host:     get("SMTP_HOST", ""),
			Port:     get("SMTP_PORT", "587"),
			Username: get("SMTP_USERNAME", ""),
			Password: getenv("SMTP_PASSWORD"),
			From:     get("MAIL_FROM", ""),
		},
		HTTP: HTTPConfig{
			ReadHeaderTimeout: 10 * time.Second,
			ReadTimeout:       30 * time.Second,
			WriteTimeout:      30 * time.Second,
			IdleTimeout:       120 * time.Second,
			ShutdownTimeout:   15 * time.Second,
		},
	}

	if cfg.JWTSecret == "" {
		return cfg, nil, errors.New("JWT_SECRET no está definido. Configuralo en backend/.env (desarrollo) " +
			"o como variable de entorno (Docker/producción) antes de iniciar el backend. " +
			"No hay clave por defecto: es intencional, por seguridad")
	}

	cfg.ReservationSweepInterval = 5 * time.Minute
	if v := get("RESERVATION_SWEEP_INTERVAL", ""); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil || d < 10*time.Second {
			warnings = append(warnings, Warning(fmt.Sprintf("RESERVATION_SWEEP_INTERVAL inválido (%q): se usa 5m", v)))
		} else {
			cfg.ReservationSweepInterval = d
		}
	}

	// Seed demo: nunca en producción, aunque SEED_DEMO_DATA quede en true.
	if cfg.SeedDemoData && cfg.IsProduction() {
		cfg.SeedDemoData = false
		warnings = append(warnings, "APP_ENV=production: se ignora SEED_DEMO_DATA=true (no se cargan datos demo)")
	}

	switch cfg.Mail.Driver {
	case "", "log", "smtp":
	default:
		warnings = append(warnings, Warning(fmt.Sprintf("MAIL_DRIVER desconocido (%q): no se enviarán emails", cfg.Mail.Driver)))
		cfg.Mail.Driver = ""
	}
	if cfg.Mail.Driver == "log" && cfg.IsProduction() {
		warnings = append(warnings, "MAIL_DRIVER=log no se permite con APP_ENV=production: no se enviarán emails")
		cfg.Mail.Driver = ""
	}
	if cfg.Mail.Driver == "smtp" && (cfg.Mail.Host == "" || cfg.Mail.From == "") {
		warnings = append(warnings, "MAIL_DRIVER=smtp sin SMTP_HOST o MAIL_FROM: no se enviarán emails")
		cfg.Mail.Driver = ""
	}
	if cfg.AdminEmail == "" || cfg.AdminPassword == "" {
		warnings = append(warnings, "ADMIN_EMAIL / ADMIN_PASSWORD no definidos: se omite la creación del admin inicial")
	}
	return cfg, warnings, nil
}
