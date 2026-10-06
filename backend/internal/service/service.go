// Package service contiene las reglas de negocio de Relojería Yampier.
//
// Cada service recibe sus dependencias por constructor (sin globals ni
// init()) y declara las interfaces chicas que necesita de los repositories
// (definidas por el consumidor, como recomienda Go): así se puede testear con
// fakes en memoria sin MongoDB.
//
// Los services devuelven errores de dominio (*domain.Error); nunca conocen
// HTTP ni códigos de estado.
package service

import (
	"errors"
	"log/slog"
	"time"

	"relojeria-yampier/internal/domain"
)

// Clock permite controlar el tiempo en los tests (vencimientos, tokens).
type Clock func() time.Time

func clockOrNow(c Clock) Clock {
	if c == nil {
		return time.Now
	}
	return c
}

func loggerOrDefault(l *slog.Logger) *slog.Logger {
	if l == nil {
		return slog.Default()
	}
	return l
}

func isNotFound(err error) bool  { return errors.Is(err, domain.ErrNotFound) }
func isDuplicate(err error) bool { return errors.Is(err, domain.ErrDuplicate) }

// Errores reutilizados con el mismo texto en varios services.
var (
	errWatchNotFound       = domain.NotFound(domain.CodeWatchNotFound, "Reloj no encontrado")
	errReservationNotFound = domain.NotFound(domain.CodeReservationNotFound, "Reserva no encontrada")
)
