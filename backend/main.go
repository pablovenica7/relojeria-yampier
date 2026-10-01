package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

func cors(next http.Handler) http.Handler {
	origin := os.Getenv("ALLOWED_ORIGIN")
	if origin == "" {
		origin = "http://localhost:5173"
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Un único origen explícito, nunca "*": el panel Admin envía un token
		// Bearer y queremos que solo el frontend configurado pueda usarlo desde
		// el navegador.
		w.Header().Set("Access-Control-Allow-Origin", origin)
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

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

// healthHandler informa si la API y MongoDB responden. No expone URI,
// credenciales ni detalles internos: solo "ok" / "unavailable".
func healthHandler(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	if err := mongoClient.Ping(ctx, nil); err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "degraded", "database": "unavailable"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "database": "ok"})
}

func sweepInterval() time.Duration {
	if v := strings.TrimSpace(os.Getenv("RESERVATION_SWEEP_INTERVAL")); v != "" {
		if d, err := time.ParseDuration(v); err == nil && d >= 10*time.Second {
			return d
		}
		log.Printf("RESERVATION_SWEEP_INTERVAL inválido (%q): se usa 5m", v)
	}
	return 5 * time.Minute
}

func main() {
	// godotenv.Load() busca un archivo .env en el directorio actual y carga
	// sus variables en el entorno del proceso. Si el archivo no existe (por
	// ejemplo en Docker, donde las variables ya llegan por environment: en
	// docker-compose) simplemente no hace nada: por eso se ignora el error.
	_ = godotenv.Load()

	secret := strings.TrimSpace(os.Getenv("JWT_SECRET"))
	if secret == "" {
		log.Fatal("JWT_SECRET no está definido. Configuralo en backend/.env (desarrollo) " +
			"o como variable de entorno (Docker/producción) antes de iniciar el backend. " +
			"No hay clave por defecto: es intencional, por seguridad.")
	}
	setJWTSecret(secret)

	connectMongo()
	defer func() { _ = mongoClient.Disconnect(context.Background()) }()

	migrateCtx, cancelMigrate := context.WithTimeout(context.Background(), 60*time.Second)
	backfillLegacyData(migrateCtx) // antes de los índices: completa modelKey/brandKey
	cancelMigrate()
	ensureIndexes()
	seedAdmin()
	seedWatches()

	appCtx, stopApp := context.WithCancel(context.Background())
	defer stopApp()
	startReservationSweeper(appCtx, sweepInterval())

	appMailer = newMailerFromEnv()
	proxies := parseTrustedProxies(os.Getenv("TRUSTED_PROXIES"))

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	srv := &http.Server{
		Addr:              ":" + port,
		Handler:           newRouter(proxies),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       120 * time.Second,
	}
	log.Println("API escuchando en :" + port)
	log.Fatal(srv.ListenAndServe())
}
