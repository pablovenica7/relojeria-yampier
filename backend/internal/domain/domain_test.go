package domain

import (
	"errors"
	"testing"
	"time"
)

func TestAvailabilityDerivedFromQuantity(t *testing.T) {
	cases := map[int]string{-3: AvailabilityOutOfStock, 0: AvailabilityOutOfStock, 1: AvailabilityLastUnit, 2: AvailabilityInStock, 50: AvailabilityInStock}
	for qty, want := range cases {
		if got := AvailabilityFor(qty); got != want {
			t.Errorf("AvailabilityFor(%d) = %q, quería %q", qty, got, want)
		}
	}
	w := Watch{StockQuantity: -1}
	w.Normalize()
	if w.StockQuantity != 0 || w.Availability != AvailabilityOutOfStock || w.Specs == nil {
		t.Errorf("Normalize no dejó un estado consistente: %+v", w)
	}
}

func TestModelKeyDetectsEquivalentModels(t *testing.T) {
	for _, m := range []string{"GA-2100-1A1", "ga 2100 1a1", " ga-2100-1a1 ", "GA21001A1"} {
		if got := ModelKeyFor(m); got != "GA21001A1" {
			t.Errorf("ModelKeyFor(%q) = %q", m, got)
		}
	}
	if ModelKeyFor("GA-2100-1A1") == ModelKeyFor("GA-2110-1A") {
		t.Error("modelos distintos no deberían compartir clave")
	}
	if BrandKeyFor("  CASIO ") != "casio" || NormalizeModel(" ga-2100 ") != "GA-2100" {
		t.Error("normalización de marca/modelo incorrecta")
	}
	if ValidModel("GA<script>") || !ValidModel("GA-2100-1A1") {
		t.Error("validación de modelo incorrecta")
	}
}

func TestImagePath(t *testing.T) {
	for _, ok := range []string{"", "/uploads/1.jpg", "/images/relojes/casio/f-91w-1.webp", "https://cdn.x/y.png"} {
		if !ValidImagePath(ok) {
			t.Errorf("%q debería ser válida", ok)
		}
	}
	for _, bad := range []string{"javascript:alert(1)", "/uploads/../secreto", "http://inseguro"} {
		if ValidImagePath(bad) {
			t.Errorf("%q no debería ser válida", bad)
		}
	}
}

func TestReservationTransitions(t *testing.T) {
	valid := [][2]string{
		{ReservationPending, ReservationConfirmed}, {ReservationPending, ReservationCancelled},
		{ReservationConfirmed, ReservationDepositPaid}, {ReservationConfirmed, ReservationCompleted},
		{ReservationConfirmed, ReservationCancelled}, {ReservationConfirmed, ReservationExpired},
		{ReservationDepositPaid, ReservationCompleted}, {ReservationDepositPaid, ReservationCancelled},
	}
	for _, tr := range valid {
		if !CanTransition(tr[0], tr[1]) {
			t.Errorf("%s -> %s debería estar permitida", tr[0], tr[1])
		}
	}
	invalid := [][2]string{
		{ReservationCompleted, ReservationPending}, {ReservationCompleted, ReservationCancelled},
		{ReservationCancelled, ReservationConfirmed}, {ReservationExpired, ReservationConfirmed},
		{ReservationPending, ReservationCompleted}, {ReservationPending, ReservationDepositPaid},
		{ReservationDepositPaid, ReservationExpired}, {ReservationDepositPaid, ReservationPending},
		{ReservationConfirmed, ReservationConfirmed},
	}
	for _, tr := range invalid {
		if CanTransition(tr[0], tr[1]) {
			t.Errorf("%s -> %s NO debería estar permitida", tr[0], tr[1])
		}
	}
	if !HoldsStock(ReservationConfirmed) || !HoldsStock(ReservationDepositPaid) || HoldsStock(ReservationPending) || HoldsStock(ReservationCompleted) {
		t.Error("solo confirmada y con seña retienen stock")
	}
}

func TestValidateExpiry(t *testing.T) {
	now := time.Now()
	past, soon, far := now.Add(-time.Hour), now.Add(time.Hour), now.AddDate(0, 0, 120)
	if ValidateExpiry(nil, now) != "" || ValidateExpiry(&soon, now) != "" {
		t.Error("vencimientos válidos rechazados")
	}
	if ValidateExpiry(&past, now) == "" || ValidateExpiry(&far, now) == "" {
		t.Error("vencimientos inválidos aceptados")
	}
}

func TestValidCUIT(t *testing.T) {
	for _, ok := range []string{"20123456786", "27000000006", "30500010912"} {
		if !ValidCUIT(ok) {
			t.Errorf("%s debería ser un CUIT válido", ok)
		}
	}
	for _, bad := range []string{"20123456787", "2012345678", "abcdefghijk", ""} {
		if ValidCUIT(bad) {
			t.Errorf("%s no debería ser válido", bad)
		}
	}
}

func TestDocumentsAndAddress(t *testing.T) {
	if d, msg := NormalizeDocument(DocDNI, "30.123.456"); msg != "" || d != "30123456" {
		t.Errorf("DNI mal normalizado: %q %q", d, msg)
	}
	if _, msg := NormalizeDocument(DocCUIT, "20-12345678-7"); msg == "" {
		t.Error("CUIT con verificador incorrecto aceptado")
	}
	if d, msg := NormalizeDocument(DocPassport, "aa 123456"); msg != "" || d != "AA123456" {
		t.Errorf("pasaporte mal normalizado: %q %q", d, msg)
	}
	a, msg := NormalizeAddress(&Address{Street: " Rivadavia ", Number: "59", PostalCode: "x5000abc", City: "Córdoba", Province: "córdoba"})
	if msg != "" || a.Province != "Córdoba" || a.PostalCode != "X5000ABC" || a.Country != "Argentina" || a.Street != "Rivadavia" {
		t.Errorf("domicilio mal normalizado: %+v %q", a, msg)
	}
	if a, msg := NormalizeAddress(&Address{}); a != nil || msg != "" {
		t.Error("un domicilio vacío es 'no informado'")
	}
	if _, msg := NormalizeAddress(&Address{Street: "Rivadavia"}); msg == "" {
		t.Error("domicilio incompleto aceptado")
	}
}

func TestPasswordAndNames(t *testing.T) {
	if ValidatePassword("secreta123") != "" || ValidatePassword("abc12") == "" || ValidatePassword("solamenteletras") == "" {
		t.Error("política de contraseña incorrecta")
	}
	if ValidatePersonName("Ana", "nombre") != "" || ValidatePersonName("Ana2", "nombre") == "" {
		t.Error("validación de nombre incorrecta")
	}
	if !ValidPhone("351 555-1234") || ValidPhone("1234") {
		t.Error("validación de teléfono incorrecta")
	}
}

func TestErrorKinds(t *testing.T) {
	err := NotFound(CodeWatchNotFound, "Reloj no encontrado")
	if !errors.Is(err, ErrNotFound) || errors.Is(err, ErrConflict) {
		t.Error("errors.Is debe reconocer el tipo de error de dominio")
	}
	var de *Error
	if !errors.As(err, &de) || de.Code != CodeWatchNotFound || de.Error() != "Reloj no encontrado" {
		t.Errorf("error de dominio mal construido: %+v", de)
	}
}
