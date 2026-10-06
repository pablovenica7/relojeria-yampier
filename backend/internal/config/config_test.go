package config

import (
	"testing"
	"time"
)

func env(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

func TestLoadRequiresJWTSecret(t *testing.T) {
	if _, _, err := load(env(map[string]string{})); err == nil {
		t.Fatal("sin JWT_SECRET el backend no debe arrancar")
	}
}

func TestLoadDefaultsAndValues(t *testing.T) {
	cfg, _, err := load(env(map[string]string{
		"JWT_SECRET": "x", "PORT": "9000", "SEED_DEMO_DATA": "TRUE", "APP_ENV": "development",
		"RESERVATION_SWEEP_INTERVAL": "30s", "APP_PUBLIC_URL": "https://yampier.com/",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Port != "9000" || !cfg.SeedDemoData || cfg.ReservationSweepInterval != 30*time.Second ||
		cfg.PublicURL != "https://yampier.com" || cfg.MongoDB != "relojeria_yampier" {
		t.Errorf("configuración inesperada: %+v", cfg)
	}
}

func TestProductionGuards(t *testing.T) {
	cfg, warnings, err := load(env(map[string]string{
		"JWT_SECRET": "x", "APP_ENV": "production", "SEED_DEMO_DATA": "true", "MAIL_DRIVER": "log",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.SeedDemoData {
		t.Error("producción nunca carga datos demo")
	}
	if cfg.Mail.Driver != "" {
		t.Error("MAIL_DRIVER=log no se permite en producción")
	}
	if len(warnings) < 2 {
		t.Errorf("deberían informarse las dos correcciones: %v", warnings)
	}
}

func TestInvalidSweepIntervalFallsBack(t *testing.T) {
	cfg, warnings, _ := load(env(map[string]string{"JWT_SECRET": "x", "RESERVATION_SWEEP_INTERVAL": "1s"}))
	if cfg.ReservationSweepInterval != 5*time.Minute || len(warnings) == 0 {
		t.Errorf("intervalo inválido debería volver a 5m con aviso: %v %v", cfg.ReservationSweepInterval, warnings)
	}
}
