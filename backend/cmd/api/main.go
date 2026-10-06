// Comando principal de la API de Relojería Yampier.
package main

import (
	"context"
	"log/slog"
	"os"
	"time"

	"github.com/joho/godotenv"

	"relojeria-yampier/internal/app"
	"relojeria-yampier/internal/config"
	"relojeria-yampier/internal/database"
)

func main() {
	log := slog.New(slog.NewTextHandler(os.Stdout, nil))
	slog.SetDefault(log)

	// En desarrollo carga backend/.env; en Docker las variables llegan por
	// environment y, si no hay archivo, esto no hace nada.
	_ = godotenv.Load()

	cfg, warnings, err := config.Load()
	if err != nil {
		log.Error("configuración inválida", "error", err)
		os.Exit(1)
	}
	for _, w := range warnings {
		log.Warn(string(w))
	}

	ctx := context.Background()
	db, err := database.Connect(ctx, cfg.MongoURI, cfg.MongoDB)
	if err != nil {
		log.Error("no se pudo iniciar la base de datos", "error", err)
		os.Exit(1)
	}
	defer func() {
		closeCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = db.Close(closeCtx)
	}()
	log.Info("conectado a MongoDB", "database", cfg.MongoDB)

	a := app.New(cfg, db, log)
	a.Prepare(ctx)
	if err := a.Run(ctx); err != nil {
		log.Error("el servidor terminó con error", "error", err)
		os.Exit(1)
	}
	log.Info("servidor detenido")
}
