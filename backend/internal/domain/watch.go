package domain

import (
	"regexp"
	"strings"
	"time"
	"unicode"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// Watch representa un reloj del catálogo.
//
// Dinero: PriceARS es un entero en pesos argentinos (sin centavos). Nunca se
// usa punto flotante para dinero. 0 significa "precio a consultar".
//
// Stock: StockQuantity es la única fuente de verdad. La disponibilidad que ve
// el cliente se deriva de ella (Availability, que no se guarda en Mongo).
//
// Archivado: un reloj con ArchivedAt != nil no aparece en el catálogo público
// pero se conserva, porque puede estar vinculado a consultas o reservas.
//
// Los tags json definen el contrato público de la API (el struct se devuelve
// tal cual); los tags bson, cómo se guarda.
type Watch struct {
	ID            primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	Brand         string             `bson:"brand" json:"brand"`
	BrandKey      string             `bson:"brandKey" json:"-"` // marca normalizada para filtrar con índice
	Name          string             `bson:"name" json:"name"`
	Model         string             `bson:"model" json:"model"`          // código comercial, ej: GA-2100-1A1
	ModelKey      string             `bson:"modelKey,omitempty" json:"-"` // clave única normalizada, ej: GA21001A1
	Gender        string             `bson:"gender" json:"gender"`        // "hombre" | "mujer"
	Description   string             `bson:"description" json:"description"`
	Image         string             `bson:"image" json:"image"` // ruta pública, ej: /uploads/archivo.jpg
	Specs         []string           `bson:"specs" json:"specs"`
	PriceARS      int64              `bson:"priceARS" json:"priceARS"`
	StockQuantity int                `bson:"stockQuantity" json:"stockQuantity"`
	Availability  string             `bson:"-" json:"availability"`
	ArchivedAt    *time.Time         `bson:"archivedAt,omitempty" json:"archivedAt,omitempty"`
	CreatedAt     time.Time          `bson:"createdAt" json:"createdAt"`
	UpdatedAt     time.Time          `bson:"updatedAt" json:"updatedAt"`
}

// Disponibilidad derivada de StockQuantity (nunca se guarda).
const (
	AvailabilityInStock    = "in_stock"     // 2 o más unidades
	AvailabilityLastUnit   = "last_unit"    // exactamente 1
	AvailabilityOutOfStock = "out_of_stock" // 0
	// Solo como filtro: cualquier cantidad >= 1.
	AvailabilityAvailable = "available"
)

// Límites de cordura (evitan overflows y errores de tipeo).
const (
	MaxPriceARS      = int64(10_000_000_000)
	MaxStockQuantity = 100_000
)

// AvailabilityFor deriva la disponibilidad de una cantidad.
func AvailabilityFor(qty int) string {
	switch {
	case qty <= 0:
		return AvailabilityOutOfStock
	case qty == 1:
		return AvailabilityLastUnit
	default:
		return AvailabilityInStock
	}
}

// Normalize completa los campos derivados antes de responder.
func (w *Watch) Normalize() {
	if w.StockQuantity < 0 {
		w.StockQuantity = 0
	}
	w.Availability = AvailabilityFor(w.StockQuantity)
	if w.Specs == nil {
		w.Specs = []string{}
	}
}

// IsArchived indica si el reloj está archivado (fuera del catálogo público).
func (w *Watch) IsArchived() bool { return w.ArchivedAt != nil }

// NormalizeModel deja el código comercial con formato consistente:
// sin espacios sobrantes y en mayúsculas ("ga-2100-1a1 " -> "GA-2100-1A1").
func NormalizeModel(s string) string { return strings.ToUpper(CollapseSpaces(s)) }

// ModelKeyFor es la clave de comparación para detectar duplicados: solo
// letras y números en mayúsculas, así "GA-2100-1A1", "ga 2100 1a1" y
// "GA21001A1" se consideran el mismo modelo.
func ModelKeyFor(model string) string {
	var b strings.Builder
	for _, r := range strings.ToUpper(model) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// BrandKeyFor normaliza la marca para filtrar con índice.
func BrandKeyFor(brand string) string { return strings.ToLower(CollapseSpaces(brand)) }

var modelCharsRe = regexp.MustCompile(`^[A-Z0-9][A-Z0-9 ./_-]*$`)

// ValidModel indica si un modelo (ya normalizado) es aceptable.
func ValidModel(model string) bool {
	return len(model) <= 60 && modelCharsRe.MatchString(model) && ModelKeyFor(model) != ""
}

// ValidImagePath acepta rutas de uploads, imágenes estáticas o URLs https
// (nunca javascript:, rutas con ".." ni otros esquemas).
func ValidImagePath(p string) bool {
	if p == "" {
		return true
	}
	if len(p) > 500 || strings.Contains(p, "..") {
		return false
	}
	return strings.HasPrefix(p, "/uploads/") || strings.HasPrefix(p, "/images/") || strings.HasPrefix(p, "https://")
}

// ValidGender: el catálogo se divide en hombre / mujer.
func ValidGender(g string) bool { return g == "hombre" || g == "mujer" }
