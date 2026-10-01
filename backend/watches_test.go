package main

import (
	"net/url"
	"strings"
	"testing"
	"time"
)

func intPtr(n int) *int { return &n }

func validWatchInput() WatchInput {
	return WatchInput{Brand: "Casio", Name: "Casio F-91W-1", Model: "F-91W-1", Gender: "hombre", StockQuantity: intPtr(2)}
}

func TestValidateWatchInput(t *testing.T) {
	cases := []struct {
		name    string
		mutate  func(*WatchInput)
		wantErr bool
	}{
		{"válido", func(*WatchInput) {}, false},
		{"válido mujer", func(in *WatchInput) { in.Gender = "mujer" }, false},
		{"nombre vacío", func(in *WatchInput) { in.Name = "  " }, true},
		{"género inválido", func(in *WatchInput) { in.Gender = "unisex" }, true},
		{"sin modelo", func(in *WatchInput) { in.Model = " " }, true},
		{"modelo con caracteres raros", func(in *WatchInput) { in.Model = "GA<script>" }, true},
		{"sin marca", func(in *WatchInput) { in.Brand = "" }, true},
		{"precio negativo", func(in *WatchInput) { in.PriceARS = -1 }, true},
		{"precio absurdo", func(in *WatchInput) { in.PriceARS = maxPriceARS + 1 }, true},
		{"stock ausente", func(in *WatchInput) { in.StockQuantity = nil }, true},
		{"stock negativo", func(in *WatchInput) { in.StockQuantity = intPtr(-1) }, true},
		{"stock cero es válido", func(in *WatchInput) { in.StockQuantity = intPtr(0) }, false},
		{"imagen javascript", func(in *WatchInput) { in.Image = "javascript:alert(1)" }, true},
		{"imagen con ..", func(in *WatchInput) { in.Image = "/uploads/../secreto" }, true},
		{"imagen estática", func(in *WatchInput) { in.Image = "/images/relojes/casio/f-91w-1.webp" }, false},
		{"spec demasiado larga", func(in *WatchInput) { in.Specs = []string{strings.Repeat("a", 201)} }, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			in := validWatchInput()
			c.mutate(&in)
			msg := validateWatchInput(&in)
			if c.wantErr && msg == "" {
				t.Errorf("esperaba un error de validación y no hubo ninguno")
			}
			if !c.wantErr && msg != "" {
				t.Errorf("no esperaba error, pero se obtuvo: %q", msg)
			}
		})
	}
}

func TestValidateWatchInputNormalizes(t *testing.T) {
	in := validWatchInput()
	in.Model = "  ga-2100-1a1 "
	in.Brand = "  Casio  "
	in.Specs = []string{" Caja 40 mm ", "", "  "}
	if msg := validateWatchInput(&in); msg != "" {
		t.Fatalf("no esperaba error: %q", msg)
	}
	if in.Model != "GA-2100-1A1" || in.Brand != "Casio" {
		t.Errorf("normalización incorrecta: model=%q brand=%q", in.Model, in.Brand)
	}
	if len(in.Specs) != 1 || in.Specs[0] != "Caja 40 mm" {
		t.Errorf("specs vacías deberían descartarse: %#v", in.Specs)
	}
}

func TestAvailabilityDerivedFromQuantity(t *testing.T) {
	cases := map[int]string{
		-3: AvailabilityOutOfStock, // nunca debería pasar, pero no se muestra como disponible
		0:  AvailabilityOutOfStock,
		1:  AvailabilityLastUnit,
		2:  AvailabilityInStock,
		50: AvailabilityInStock,
	}
	for qty, want := range cases {
		if got := availabilityFor(qty); got != want {
			t.Errorf("availabilityFor(%d) = %q, quería %q", qty, got, want)
		}
	}
	w := Watch{StockQuantity: -1}
	normalizeWatch(&w)
	if w.StockQuantity != 0 || w.Availability != AvailabilityOutOfStock || w.Specs == nil {
		t.Errorf("normalizeWatch no dejó un estado consistente: %+v", w)
	}
}

func TestModelKeyDetectsEquivalentModels(t *testing.T) {
	same := []string{"GA-2100-1A1", "ga 2100 1a1", " ga-2100-1a1 ", "GA21001A1"}
	for _, m := range same {
		if got := modelKeyFor(m); got != "GA21001A1" {
			t.Errorf("modelKeyFor(%q) = %q", m, got)
		}
	}
	if modelKeyFor("GA-2100-1A1") == modelKeyFor("GA-2110-1A") {
		t.Error("modelos distintos no deberían compartir clave")
	}
	if brandKeyFor("  CASIO ") != "casio" {
		t.Error("brandKey debería normalizar a minúsculas sin espacios")
	}
}

func TestParseCatalogQuery(t *testing.T) {
	q, msg := parseCatalogQuery(url.Values{
		"page": {"2"}, "limit": {"10"}, "search": {"ga2100"}, "availability": {"in_stock"},
		"brand": {"Casio"}, "sort": {"price_asc"},
	}, false)
	if msg != "" {
		t.Fatalf("no esperaba error: %q", msg)
	}
	if q.Page != 2 || q.Limit != 10 || q.BrandKey != "casio" || q.Status != "active" {
		t.Errorf("parseo incorrecto: %+v", q)
	}
	f := q.filter()
	if f["brandKey"] != "casio" || f["$or"] == nil || f["stockQuantity"] == nil {
		t.Errorf("filtro incompleto: %#v", f)
	}

	invalid := []url.Values{
		{"limit": {"999999"}},
		{"limit": {"0"}},
		{"page": {"abc"}},
		{"sort": {"$where"}},
		{"availability": {"todos"}},
		{"gender": {"x"}},
		{"status": {"archived"}}, // el público no puede ver archivados
	}
	for _, v := range invalid {
		if _, msg := parseCatalogQuery(v, false); msg == "" {
			t.Errorf("esperaba error para %v", v)
		}
	}
	if q, msg := parseCatalogQuery(url.Values{"status": {"archived"}}, true); msg != "" || q.Status != "archived" {
		t.Errorf("el Admin debería poder filtrar archivados (msg=%q)", msg)
	}
}

func TestNewPaged(t *testing.T) {
	p := newPaged([]int{1, 2}, 1, 24, 120)
	if p.Pages != 5 || p.Total != 120 {
		t.Errorf("páginas calculadas mal: %+v", p)
	}
	if newPaged([]int{}, 1, 24, 0).Pages != 0 {
		t.Error("sin resultados debería haber 0 páginas")
	}
}

func TestSeedBlockedInProduction(t *testing.T) {
	if seedEnabled("production", "true") {
		t.Error("APP_ENV=production nunca debe cargar datos demo")
	}
	if !seedEnabled("development", "true") || !seedEnabled("", "TRUE") {
		t.Error("en desarrollo con SEED_DEMO_DATA=true debería cargar")
	}
	if seedEnabled("development", "false") || seedEnabled("development", "") {
		t.Error("sin SEED_DEMO_DATA=true no debería cargar")
	}
}

func TestDemoCasioWatchesAreValid(t *testing.T) {
	keys := map[string]bool{}
	for _, w := range demoCasioWatches(time.Now()) {
		in := WatchInput{Brand: w.Brand, Name: w.Name, Model: w.Model, Gender: w.Gender,
			Image: w.Image, Specs: w.Specs, PriceARS: w.PriceARS, StockQuantity: intPtr(w.StockQuantity)}
		if msg := validateWatchInput(&in); msg != "" {
			t.Errorf("%s: dato de demo inválido: %s", w.Name, msg)
		}
		if w.ModelKey == "" || w.BrandKey != "casio" {
			t.Errorf("%s: faltan claves normalizadas", w.Name)
		}
		if keys[w.ModelKey] {
			t.Errorf("modelo demo duplicado: %s", w.Model)
		}
		keys[w.ModelKey] = true
	}
}

func TestLegacyConversions(t *testing.T) {
	p := 149999.6
	if legacyPriceARS(&p) != 150000 || legacyPriceARS(nil) != 0 {
		t.Error("conversión de precio legado incorrecta")
	}
	if legacyStockQuantity("sin_stock") != 0 || legacyStockQuantity("ultima_unidad") != 1 ||
		legacyStockQuantity("en_stock") != 1 || legacyStockQuantity("") != 0 {
		t.Error("conversión de stock legado incorrecta")
	}
}

func TestJWTRoundTrip(t *testing.T) {
	setJWTSecret("clave-de-test-no-usar-en-produccion")

	token, err := generateAdminToken(&AdminUser{Email: "admin@example.com"})
	if err != nil {
		t.Fatalf("generateAdminToken devolvió error: %v", err)
	}

	claims, err := parseToken(token)
	if err != nil {
		t.Fatalf("parseToken devolvió error para un token recién generado: %v", err)
	}
	if claims.Email != "admin@example.com" {
		t.Errorf("email esperado 'admin@example.com', obtuvo %q", claims.Email)
	}
}

func TestJWTRejectsTamperedToken(t *testing.T) {
	setJWTSecret("clave-de-test-no-usar-en-produccion")

	token, _ := generateAdminToken(&AdminUser{Email: "admin@example.com"})
	tampered := token + "x"

	if _, err := parseToken(tampered); err == nil {
		t.Error("un token alterado no debería pasar la validación")
	}
}
