package main

import (
	"math"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"unicode"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// Disponibilidad derivada de StockQuantity (nunca se guarda).
const (
	AvailabilityInStock    = "in_stock"     // 2 o más unidades
	AvailabilityLastUnit   = "last_unit"    // exactamente 1
	AvailabilityOutOfStock = "out_of_stock" // 0
	// Solo como filtro: cualquier cantidad >= 1.
	AvailabilityAvailable = "available"
)

const (
	maxPriceARS      = int64(10_000_000_000) // tope de cordura, evita overflows y errores de tipeo
	maxStockQuantity = 100_000
	defaultPageLimit = 24
	maxPageLimit     = 100
)

func availabilityFor(qty int) string {
	switch {
	case qty <= 0:
		return AvailabilityOutOfStock
	case qty == 1:
		return AvailabilityLastUnit
	default:
		return AvailabilityInStock
	}
}

// normalizeWatch completa los campos derivados antes de responder.
func normalizeWatch(w *Watch) {
	if w.StockQuantity < 0 {
		w.StockQuantity = 0
	}
	w.Availability = availabilityFor(w.StockQuantity)
	if w.Specs == nil {
		w.Specs = []string{}
	}
}

var spacesRe = regexp.MustCompile(`\s+`)

func collapseSpaces(s string) string {
	return spacesRe.ReplaceAllString(strings.TrimSpace(s), " ")
}

// normalizeModel deja el código comercial con formato consistente:
// sin espacios sobrantes y en mayúsculas ("ga-2100-1a1 " -> "GA-2100-1A1").
func normalizeModel(s string) string {
	return strings.ToUpper(collapseSpaces(s))
}

// modelKeyFor es la clave de comparación para detectar duplicados: solo
// letras y números en mayúsculas, así "GA-2100-1A1", "ga 2100 1a1" y
// "GA21001A1" se consideran el mismo modelo.
func modelKeyFor(model string) string {
	var b strings.Builder
	for _, r := range strings.ToUpper(model) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func brandKeyFor(brand string) string {
	return strings.ToLower(collapseSpaces(brand))
}

var modelCharsRe = regexp.MustCompile(`^[A-Z0-9][A-Z0-9 ./_-]*$`)

// validateWatchInput normaliza y valida el cuerpo de alta/edición de un reloj.
// Toda regla importante vive acá, no solo en el frontend.
func validateWatchInput(in *WatchInput) string {
	in.Brand = collapseSpaces(in.Brand)
	in.Name = collapseSpaces(in.Name)
	in.Model = normalizeModel(in.Model)
	in.Description = strings.TrimSpace(in.Description)
	in.Image = strings.TrimSpace(in.Image)
	in.StockMoveReason = strings.TrimSpace(in.StockMoveReason)
	if in.StockMoveType == "" {
		in.StockMoveType = MoveManualAdjustment
	}

	specs := make([]string, 0, len(in.Specs))
	for _, s := range in.Specs {
		if s = strings.TrimSpace(s); s != "" {
			specs = append(specs, s)
		}
	}
	in.Specs = specs

	switch {
	case in.Name == "":
		return "El nombre es obligatorio"
	case len(in.Name) > 120:
		return "El nombre es demasiado largo (máximo 120 caracteres)"
	case in.Brand == "":
		return "La marca es obligatoria"
	case len(in.Brand) > 60:
		return "La marca es demasiado larga"
	case in.Model == "":
		return "El modelo es obligatorio"
	case len(in.Model) > 60 || !modelCharsRe.MatchString(in.Model) || modelKeyFor(in.Model) == "":
		return "El modelo solo puede tener letras, números, espacios y - . / _"
	case in.Gender != "hombre" && in.Gender != "mujer":
		return "El género debe ser 'hombre' o 'mujer'"
	case len(in.Description) > 4000:
		return "La descripción es demasiado larga"
	case len(in.Specs) > 40:
		return "Demasiadas especificaciones (máximo 40)"
	case in.PriceARS < 0:
		return "El precio no puede ser negativo"
	case in.PriceARS > maxPriceARS:
		return "El precio es demasiado alto"
	case in.StockQuantity == nil:
		return "La cantidad en stock es obligatoria"
	case *in.StockQuantity < 0:
		return "El stock no puede ser negativo"
	case *in.StockQuantity > maxStockQuantity:
		return "La cantidad en stock es demasiado alta"
	case in.StockQuantityBase != nil && *in.StockQuantityBase < 0:
		return "Stock base inválido"
	case in.StockMoveType != "" && !validManualMoveType(in.StockMoveType):
		return "Tipo de movimiento de stock inválido"
	case len(in.StockMoveReason) > 300:
		return "El motivo del movimiento es demasiado largo"
	case !validImagePath(in.Image):
		return "La imagen debe ser una ruta /uploads/..., /images/... o una URL https"
	}
	for _, s := range in.Specs {
		if len(s) > 200 {
			return "Cada especificación debe tener como máximo 200 caracteres"
		}
	}
	return ""
}

func validImagePath(p string) bool {
	if p == "" {
		return true
	}
	if len(p) > 500 || strings.Contains(p, "..") {
		return false
	}
	return strings.HasPrefix(p, "/uploads/") || strings.HasPrefix(p, "/images/") || strings.HasPrefix(p, "https://")
}

// ---------- Búsqueda, filtros y paginación del catálogo ----------

type catalogQuery struct {
	Search       string
	BrandKey     string
	Gender       string
	Availability string
	Sort         string
	Status       string // solo Admin: active | archived | all
	Page         int
	Limit        int
}

var catalogSorts = map[string]bson.D{
	"newest":     {{Key: "createdAt", Value: -1}, {Key: "_id", Value: -1}},
	"name_asc":   {{Key: "name", Value: 1}, {Key: "_id", Value: 1}},
	"name_desc":  {{Key: "name", Value: -1}, {Key: "_id", Value: 1}},
	"price_asc":  {{Key: "priceARS", Value: 1}, {Key: "_id", Value: 1}},
	"price_desc": {{Key: "priceARS", Value: -1}, {Key: "_id", Value: 1}},
}

// parseCatalogQuery valida los parámetros. Devuelve un mensaje de error
// (para un 400) si alguno es inválido. allowStatus habilita el filtro de
// archivados, que solo tiene sentido en el panel Admin.
func parseCatalogQuery(v url.Values, allowStatus bool) (catalogQuery, string) {
	q := catalogQuery{
		Search:       strings.TrimSpace(v.Get("search")),
		BrandKey:     brandKeyFor(v.Get("brand")),
		Gender:       strings.TrimSpace(v.Get("gender")),
		Availability: strings.TrimSpace(v.Get("availability")),
		Sort:         strings.TrimSpace(v.Get("sort")),
		Status:       "active",
		Page:         1,
		Limit:        defaultPageLimit,
	}
	if len(q.Search) > 80 {
		return q, "La búsqueda es demasiado larga"
	}
	if q.Gender != "" && q.Gender != "hombre" && q.Gender != "mujer" {
		return q, "Género inválido"
	}
	switch q.Availability {
	case "", AvailabilityInStock, AvailabilityLastUnit, AvailabilityOutOfStock, AvailabilityAvailable:
	default:
		return q, "Disponibilidad inválida"
	}
	if q.Sort == "" {
		q.Sort = "newest"
	}
	if _, ok := catalogSorts[q.Sort]; !ok {
		return q, "Orden inválido"
	}
	if s := v.Get("status"); s != "" {
		if !allowStatus || (s != "active" && s != "archived" && s != "all") {
			return q, "Estado inválido"
		}
		q.Status = s
	}
	if p := v.Get("page"); p != "" {
		n, err := strconv.Atoi(p)
		if err != nil || n < 1 || n > 100_000 {
			return q, "Página inválida"
		}
		q.Page = n
	}
	if l := v.Get("limit"); l != "" {
		n, err := strconv.Atoi(l)
		if err != nil || n < 1 || n > maxPageLimit {
			return q, "El límite debe estar entre 1 y " + strconv.Itoa(maxPageLimit)
		}
		q.Limit = n
	}
	return q, ""
}

// scopeFilter filtra solo por estado de archivado (base de los contadores).
func (q catalogQuery) scopeFilter() bson.M {
	switch q.Status {
	case "archived":
		return bson.M{"archivedAt": bson.M{"$ne": nil}}
	case "all":
		return bson.M{}
	default:
		// {archivedAt: null} también encuentra documentos viejos sin el campo.
		return bson.M{"archivedAt": nil}
	}
}

func availabilityFilter(a string) bson.M {
	switch a {
	case AvailabilityInStock:
		return bson.M{"$gte": 2}
	case AvailabilityLastUnit:
		return bson.M{"$eq": 1}
	case AvailabilityOutOfStock:
		return bson.M{"$lte": 0}
	case AvailabilityAvailable:
		return bson.M{"$gte": 1}
	}
	return nil
}

func (q catalogQuery) filter() bson.M {
	f := q.scopeFilter()
	if q.BrandKey != "" {
		f["brandKey"] = q.BrandKey
	}
	if q.Gender != "" {
		f["gender"] = q.Gender
	}
	if af := availabilityFilter(q.Availability); af != nil {
		f["stockQuantity"] = af
	}
	if q.Search != "" {
		// Nombre/marca: texto literal sin distinguir mayúsculas.
		// Modelo: se compara la clave normalizada, así "ga2100" encuentra
		// "GA-2100-1A1". Las regex se construyen con QuoteMeta: el texto del
		// usuario nunca se interpreta como expresión regular.
		or := bson.A{
			bson.M{"name": bson.M{"$regex": regexp.QuoteMeta(q.Search), "$options": "i"}},
			bson.M{"brand": bson.M{"$regex": regexp.QuoteMeta(q.Search), "$options": "i"}},
		}
		if key := modelKeyFor(q.Search); key != "" {
			or = append(or, bson.M{"modelKey": bson.M{"$regex": regexp.QuoteMeta(key)}})
		}
		f["$or"] = or
	}
	return f
}

func (q catalogQuery) findOptions() *options.FindOptions {
	opts := options.Find().
		SetSort(catalogSorts[q.Sort]).
		SetSkip(int64((q.Page - 1) * q.Limit)).
		SetLimit(int64(q.Limit))
	if strings.HasPrefix(q.Sort, "name_") {
		// Collation en español solo al ordenar por nombre (acentos y
		// mayúsculas en orden correcto). No se usa siempre porque una
		// collation distinta a la del índice impide usarlo en las igualdades
		// de strings (brandKey, gender).
		opts.SetCollation(&options.Collation{Locale: "es", Strength: 1})
	}
	return opts
}

// paged es el formato de respuesta de los listados paginados.
type paged[T any] struct {
	Items []T   `json:"items"`
	Page  int   `json:"page"`
	Limit int   `json:"limit"`
	Total int64 `json:"total"`
	Pages int   `json:"pages"`
}

func newPaged[T any](items []T, page, limit int, total int64) paged[T] {
	pages := int(math.Ceil(float64(total) / float64(limit)))
	return paged[T]{Items: items, Page: page, Limit: limit, Total: total, Pages: pages}
}
