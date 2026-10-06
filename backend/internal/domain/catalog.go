package domain

// Paginación de listados.
const (
	DefaultPageLimit = 24
	MaxPageLimit     = 100
)

// Ordenamientos válidos del catálogo.
const (
	SortNewest    = "newest"
	SortNameAsc   = "name_asc"
	SortNameDesc  = "name_desc"
	SortPriceAsc  = "price_asc"
	SortPriceDesc = "price_desc"
)

// ValidSort indica si el orden pedido existe.
func ValidSort(s string) bool {
	switch s {
	case SortNewest, SortNameAsc, SortNameDesc, SortPriceAsc, SortPriceDesc:
		return true
	}
	return false
}

// Alcance por estado de archivado (solo Admin).
const (
	ScopeActive   = "active"
	ScopeArchived = "archived"
	ScopeAll      = "all"
)

// CatalogFilter describe una búsqueda del catálogo, ya validada.
type CatalogFilter struct {
	Search       string
	BrandKey     string
	Gender       string
	Availability string
	Sort         string
	Scope        string // ScopeActive | ScopeArchived | ScopeAll
	Page         int
	Limit        int
}

// CatalogStats son los contadores del listado del Admin.
type CatalogStats struct {
	Total      int64 `json:"total"`
	OutOfStock int64 `json:"outOfStock"`
	LastUnit   int64 `json:"lastUnit"`
}
