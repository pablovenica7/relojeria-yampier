package main

import (
	"strings"
	"testing"
)

func TestValidateInquiry(t *testing.T) {
	cases := []struct {
		name    string
		in      InquiryInput
		wantErr bool
	}{
		{"válida", InquiryInput{Name: "Juan", Email: "juan@example.com", Message: "Quiero consultar por un reloj"}, false},
		{"válida con reloj", InquiryInput{Name: "Juan", Email: "juan@example.com", Message: "Quiero consultar por un reloj", WatchID: "6abe975b32d148f8dfe67813"}, false},
		{"reloj inválido", InquiryInput{Name: "Juan", Email: "juan@example.com", Message: "Quiero consultar por un reloj", WatchID: "no-es-un-id"}, true},
		{"nombre vacío", InquiryInput{Name: "  ", Email: "juan@example.com", Message: "Quiero consultar por un reloj"}, true},
		{"email inválido", InquiryInput{Name: "Juan", Email: "no-es-un-email", Message: "Quiero consultar por un reloj"}, true},
		{"mensaje muy corto", InquiryInput{Name: "Juan", Email: "juan@example.com", Message: "hola"}, true},
		{"mensaje muy largo", InquiryInput{Name: "Juan", Email: "juan@example.com", Message: strings.Repeat("a", 2001)}, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			in := c.in
			msg := validateInquiry(&in)
			if c.wantErr && msg == "" {
				t.Errorf("esperaba un error de validación y no hubo ninguno")
			}
			if !c.wantErr && msg != "" {
				t.Errorf("no esperaba error, pero se obtuvo: %q", msg)
			}
		})
	}
}

func TestValidateInquiryTrimsFields(t *testing.T) {
	in := InquiryInput{Name: "  Juan  ", Email: " juan@example.com ", Message: "  Quiero consultar por un reloj  "}
	if msg := validateInquiry(&in); msg != "" {
		t.Fatalf("no esperaba error: %q", msg)
	}
	if in.Name != "Juan" {
		t.Errorf("el nombre debería quedar sin espacios, obtuvo %q", in.Name)
	}
}

func TestInquiryStatuses(t *testing.T) {
	for _, s := range []string{InquiryNew, InquiryContacted, InquiryClosed} {
		if !validInquiryStatus(s) {
			t.Errorf("%q debería ser válido", s)
		}
	}
	if validInquiryStatus("read") || validInquiryStatus("") {
		t.Error("estados desconocidos deben rechazarse")
	}
}
