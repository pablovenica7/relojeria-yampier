// Package controller contiene los handlers HTTP. Cada handler solo: lee el
// request, valida el FORMATO (JSON, ids, query params), obtiene el usuario
// autenticado del contexto, llama a un service y escribe la respuesta. No hay
// consultas a Mongo ni reglas de negocio acá.
package controller

import (
	"net/url"
	"strconv"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"

	"relojeria-yampier/internal/domain"
)

// parseCatalogQuery valida los query params del catálogo. allowScope
// habilita status=active|archived|all (solo Admin).
func parseCatalogQuery(v url.Values, allowScope bool) (domain.CatalogFilter, string) {
	f := domain.CatalogFilter{
		Search:       strings.TrimSpace(v.Get("search")),
		BrandKey:     domain.BrandKeyFor(v.Get("brand")),
		Gender:       strings.TrimSpace(v.Get("gender")),
		Availability: strings.TrimSpace(v.Get("availability")),
		Sort:         strings.TrimSpace(v.Get("sort")),
		Scope:        domain.ScopeActive,
		Page:         1,
		Limit:        domain.DefaultPageLimit,
	}
	if len(f.Search) > 80 {
		return f, "La búsqueda es demasiado larga"
	}
	if f.Gender != "" && !domain.ValidGender(f.Gender) {
		return f, "Género inválido"
	}
	switch f.Availability {
	case "", domain.AvailabilityInStock, domain.AvailabilityLastUnit, domain.AvailabilityOutOfStock, domain.AvailabilityAvailable:
	default:
		return f, "Disponibilidad inválida"
	}
	if f.Sort == "" {
		f.Sort = domain.SortNewest
	}
	if !domain.ValidSort(f.Sort) {
		return f, "Orden inválido"
	}
	if s := v.Get("status"); s != "" {
		if !allowScope || (s != domain.ScopeActive && s != domain.ScopeArchived && s != domain.ScopeAll) {
			return f, "Estado inválido"
		}
		f.Scope = s
	}
	if p := v.Get("page"); p != "" {
		n, err := strconv.Atoi(p)
		if err != nil || n < 1 || n > 100_000 {
			return f, "Página inválida"
		}
		f.Page = n
	}
	if l := v.Get("limit"); l != "" {
		n, err := strconv.Atoi(l)
		if err != nil || n < 1 || n > domain.MaxPageLimit {
			return f, "El límite debe estar entre 1 y " + strconv.Itoa(domain.MaxPageLimit)
		}
		f.Limit = n
	}
	return f, ""
}

// parsePage valida page/limit de los listados del Admin.
func parsePage(v url.Values, defLimit int, limitMsg string) (int, int, string) {
	page, limit := 1, defLimit
	if s := v.Get("page"); s != "" {
		n, err := strconv.Atoi(s)
		if err != nil || n < 1 || n > 100_000 {
			return 0, 0, "Página inválida"
		}
		page = n
	}
	if s := v.Get("limit"); s != "" {
		n, err := strconv.Atoi(s)
		if err != nil || n < 1 || n > domain.MaxPageLimit {
			return 0, 0, limitMsg
		}
		limit = n
	}
	return page, limit, ""
}

// parseDateRange traduce from/to (AAAA-MM-DD, hora local; "to" incluye el
// día completo) a un rango [from, to).
func parseDateRange(v url.Values) (*time.Time, *time.Time, string) {
	var from, to *time.Time
	for _, p := range []struct {
		name string
		dst  **time.Time
	}{{"from", &from}, {"to", &to}} {
		s := v.Get(p.name)
		if s == "" {
			continue
		}
		d, err := time.ParseInLocation("2006-01-02", s, time.Local)
		if err != nil {
			return nil, nil, "Fecha inválida (usá AAAA-MM-DD)"
		}
		if p.name == "to" {
			d = d.AddDate(0, 0, 1)
		}
		*p.dst = &d
	}
	return from, to, ""
}

// optionalObjectID lee un id opcional de los query params.
func optionalObjectID(v url.Values, name string) (*primitive.ObjectID, bool) {
	s := v.Get(name)
	if s == "" {
		return nil, true
	}
	id, err := primitive.ObjectIDFromHex(s)
	if err != nil {
		return nil, false
	}
	return &id, true
}
