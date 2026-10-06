package database

import (
	"testing"
	"time"

	"relojeria-yampier/internal/domain"
)

func TestLegacyConversions(t *testing.T) {
	p := 149999.6
	if LegacyPriceARS(&p) != 150000 || LegacyPriceARS(nil) != 0 {
		t.Error("conversión de precio legado incorrecta")
	}
	if LegacyStockQuantity("sin_stock") != 0 || LegacyStockQuantity("ultima_unidad") != 1 ||
		LegacyStockQuantity("en_stock") != 1 || LegacyStockQuantity("") != 0 {
		t.Error("conversión de stock legado incorrecta")
	}
}

func TestDemoCasioWatchesAreConsistent(t *testing.T) {
	keys := map[string]bool{}
	for _, w := range DemoCasioWatches(time.Now()) {
		if w.BrandKey != "casio" || w.ModelKey == "" || !domain.ValidModel(w.Model) ||
			!domain.ValidImagePath(w.Image) || len(w.Specs) == 0 || w.StockQuantity < 0 {
			t.Errorf("dato demo inválido: %+v", w)
		}
		if keys[w.ModelKey] {
			t.Errorf("modelo demo duplicado: %s", w.Model)
		}
		keys[w.ModelKey] = true
	}
}
