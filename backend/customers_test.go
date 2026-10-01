package main

import (
	"encoding/json"
	"strings"
	"testing"
)

func validCustomerInput() CustomerInput {
	return CustomerInput{
		FirstName: "Ana", LastName: "Pérez", Email: "  Ana.Perez@Example.COM ", Phone: "351 555-1234",
		Password: "secreta123", DocumentType: DocDNI, DocumentNumber: "30.123.456",
		TaxCondition: TaxConsumerFinal, PrivacyAccepted: true,
	}
}

func TestValidCUIT(t *testing.T) {
	for _, ok := range []string{"20123456786", "27000000006", "30500010912"} {
		if !validCUIT(ok) {
			t.Errorf("%s debería ser un CUIT válido", ok)
		}
	}
	for _, bad := range []string{"20123456787", "2012345678", "abcdefghijk", ""} {
		if validCUIT(bad) {
			t.Errorf("%s no debería ser válido", bad)
		}
	}
}

func TestValidateCustomerInputNormalizes(t *testing.T) {
	in := validCustomerInput()
	if msg := validateCustomerInput(&in, true); msg != "" {
		t.Fatalf("entrada válida rechazada: %q", msg)
	}
	if in.Email != "ana.perez@example.com" {
		t.Errorf("el email debe normalizarse a minúsculas sin espacios: %q", in.Email)
	}
	if in.DocumentNumber != "30123456" {
		t.Errorf("el DNI debe guardarse solo con dígitos: %q", in.DocumentNumber)
	}
	if in.BillingAddress != nil || in.TaxIDType != "" {
		t.Error("sin domicilio ni dato fiscal no deberían guardarse valores vacíos")
	}
}

func TestValidateCustomerInputRules(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*CustomerInput)
		ok     bool
	}{
		{"sin aceptar privacidad", func(in *CustomerInput) { in.PrivacyAccepted = false }, false},
		{"contraseña corta", func(in *CustomerInput) { in.Password = "abc12" }, false},
		{"contraseña sin números", func(in *CustomerInput) { in.Password = "solamenteletras" }, false},
		{"nombre con números", func(in *CustomerInput) { in.FirstName = "Ana2" }, false},
		{"email inválido", func(in *CustomerInput) { in.Email = "no-email" }, false},
		{"teléfono corto", func(in *CustomerInput) { in.Phone = "1234" }, false},
		{"DNI corto", func(in *CustomerInput) { in.DocumentNumber = "123" }, false},
		{"tipo de documento inválido", func(in *CustomerInput) { in.DocumentType = "libreta" }, false},
		{"pasaporte", func(in *CustomerInput) { in.DocumentType, in.DocumentNumber = DocPassport, "aa 123456" }, true},
		{"CUIT como documento con verificador mal", func(in *CustomerInput) { in.DocumentType, in.DocumentNumber = DocCUIT, "20-12345678-7" }, false},
		{"condición fiscal inválida", func(in *CustomerInput) { in.TaxCondition = "vip" }, false},
		{"monotributo sin CUIT", func(in *CustomerInput) { in.TaxCondition = TaxMonotributo }, false},
		{"monotributo con CUIT", func(in *CustomerInput) {
			in.TaxCondition, in.TaxIDType, in.TaxID = TaxMonotributo, DocCUIT, "20-12345678-6"
		}, true},
		{"CUIT sin tipo", func(in *CustomerInput) { in.TaxID = "20123456786" }, false},
		{"domicilio completo", func(in *CustomerInput) {
			in.BillingAddress = &Address{Street: "Rivadavia", Number: "59", PostalCode: "x5000abc", City: "Córdoba", Province: "córdoba"}
		}, true},
		{"domicilio incompleto", func(in *CustomerInput) { in.BillingAddress = &Address{Street: "Rivadavia"} }, false},
		{"provincia inexistente", func(in *CustomerInput) {
			in.BillingAddress = &Address{Street: "Rivadavia", Number: "59", PostalCode: "5000", City: "Córdoba", Province: "Narnia"}
		}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			in := validCustomerInput()
			c.mutate(&in)
			msg := validateCustomerInput(&in, true)
			if c.ok && msg != "" {
				t.Errorf("esperaba válido, obtuvo %q", msg)
			}
			if !c.ok && msg == "" {
				t.Error("esperaba un error de validación")
			}
		})
	}
}

func TestAddressIsNormalized(t *testing.T) {
	in := validCustomerInput()
	in.BillingAddress = &Address{Street: " Rivadavia ", Number: "59", PostalCode: "x5000abc", City: "Córdoba", Province: "córdoba"}
	if msg := validateCustomerInput(&in, true); msg != "" {
		t.Fatal(msg)
	}
	a := in.BillingAddress
	if a.Province != "Córdoba" || a.PostalCode != "X5000ABC" || a.Country != "Argentina" || a.Street != "Rivadavia" {
		t.Errorf("domicilio mal normalizado: %+v", a)
	}
}

func TestCustomerJSONNeverExposesPassword(t *testing.T) {
	c := Customer{FirstName: "Ana", LastName: "Pérez", PasswordHash: "$2a$10$secreto", CustomerType: CustomerIndividual}
	b, _ := json.Marshal(c)
	if strings.Contains(string(b), "secreto") || strings.Contains(string(b), "passwordHash") {
		t.Errorf("el JSON del cliente expone el hash: %s", b)
	}
	if !strings.Contains(string(b), `"legalName":"Ana Pérez"`) {
		t.Errorf("falta legalName: %s", b)
	}
}

func TestTokensAreRoleScoped(t *testing.T) {
	setJWTSecret("clave-de-test")
	adminTok, _ := generateAdminToken(&AdminUser{Email: "admin@example.com"})
	c, err := parseToken(adminTok)
	if err != nil || c.Role != RoleAdmin {
		t.Fatalf("token admin sin rol: %v %+v", err, c)
	}
	custTok, _ := generateCustomerToken(&Customer{ID: [12]byte{1}})
	c, err = parseToken(custTok)
	if err != nil || c.Role != RoleCustomer || c.Email != "" {
		t.Fatalf("token de cliente incorrecto (no debe llevar datos personales): %v %+v", err, c)
	}
}
