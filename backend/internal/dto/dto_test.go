package dto

import (
	"encoding/json"
	"strings"
	"testing"

	"relojeria-yampier/internal/domain"
)

// El JSON de un cliente es contrato con el frontend y NUNCA debe exponer
// datos internos.
func TestCustomerResponseContract(t *testing.T) {
	c := &domain.Customer{FirstName: "Ana", LastName: "Pérez", PasswordHash: "$2a$10$secreto", TokenVersion: 7,
		CustomerType: domain.CustomerIndividual, Active: true}
	b, _ := json.Marshal(NewCustomerResponse(c))
	s := string(b)
	for _, forbidden := range []string{"secreto", "passwordHash", "tokenVersion", `"active"`} {
		if strings.Contains(s, forbidden) {
			t.Errorf("el JSON del cliente expone %q: %s", forbidden, s)
		}
	}
	for _, key := range []string{`"legalName":"Ana Pérez"`, `"taxIdType":""`, `"billingAddress":null`, `"firstName"`, `"privacyVersion"`} {
		if !strings.Contains(s, key) {
			t.Errorf("falta %s en el contrato: %s", key, s)
		}
	}

	ab, _ := json.Marshal(NewAdminCustomerResponse(c))
	if !strings.Contains(string(ab), `"active":true`) || !strings.Contains(string(ab), `"legalName"`) {
		t.Errorf("la vista del Admin debe incluir active y los datos del cliente: %s", ab)
	}
}

func TestNewPage(t *testing.T) {
	p := NewPage([]int{1, 2}, 1, 24, 120)
	if p.Pages != 5 || p.Total != 120 {
		t.Errorf("páginas calculadas mal: %+v", p)
	}
	empty := NewPage[int](nil, 1, 24, 0)
	b, _ := json.Marshal(empty)
	if empty.Pages != 0 || !strings.Contains(string(b), `"items":[]`) {
		t.Errorf("sin resultados debe devolver items vacío (no null): %s", b)
	}
}

func TestCustomerReservationHidesInternals(t *testing.T) {
	r := &domain.Reservation{Status: domain.ReservationPending, Notes: "nota interna", StockHeld: true}
	b, _ := json.Marshal(NewCustomerReservationResponse(r))
	if strings.Contains(string(b), "nota interna") || strings.Contains(string(b), "stockHeld") {
		t.Errorf("la vista del cliente expone datos internos: %s", b)
	}
	if !strings.Contains(string(b), `"canCancel":true`) {
		t.Error("una pendiente debe poder cancelarse")
	}
}
