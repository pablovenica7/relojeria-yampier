package controller

import (
	"net/http"
	"strings"

	"relojeria-yampier/internal/domain"
	"relojeria-yampier/internal/dto"
	"relojeria-yampier/internal/httpx"
	"relojeria-yampier/internal/service"
)

// AdminHandler agrupa las vistas operativas del panel.
type AdminHandler struct {
	dashboard *service.DashboardService
	customers *service.CustomerService
	stock     *service.StockService
	uploads   *service.UploadService
}

func NewAdminHandler(dashboard *service.DashboardService, customers *service.CustomerService, stock *service.StockService, uploads *service.UploadService) *AdminHandler {
	return &AdminHandler{dashboard: dashboard, customers: customers, stock: stock, uploads: uploads}
}

// Dashboard: GET /api/admin/dashboard
func (h *AdminHandler) Dashboard(w http.ResponseWriter, r *http.Request) {
	d, err := h.dashboard.Summary(r.Context())
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, d)
}

// Customers: GET /api/admin/customers?search=&status=active|inactive|all&page=&limit=
func (h *AdminHandler) Customers(w http.ResponseWriter, r *http.Request) {
	v := r.URL.Query()
	var f domain.CustomerFilter
	switch v.Get("status") {
	case "", "all":
	case "active", "inactive":
		active := v.Get("status") == "active"
		f.Active = &active
	default:
		httpx.WriteMessage(w, http.StatusBadRequest, "", "Estado inválido")
		return
	}
	f.Search = strings.TrimSpace(v.Get("search"))
	if len(f.Search) > 80 {
		httpx.WriteMessage(w, http.StatusBadRequest, "", "La búsqueda es demasiado larga")
		return
	}
	page, limit, msg := parsePage(v, 25, "Límite inválido (1 a 100)")
	if msg != "" {
		httpx.WriteMessage(w, http.StatusBadRequest, "", msg)
		return
	}
	out, err := h.customers.ListForAdmin(r.Context(), f, page, limit)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, out)
}

// DeactivateCustomer / ReactivateCustomer: PATCH /api/admin/customers/{id}/deactivate|reactivate
func (h *AdminHandler) DeactivateCustomer(w http.ResponseWriter, r *http.Request) {
	h.setCustomerActive(w, r, false)
}

func (h *AdminHandler) ReactivateCustomer(w http.ResponseWriter, r *http.Request) {
	h.setCustomerActive(w, r, true)
}

func (h *AdminHandler) setCustomerActive(w http.ResponseWriter, r *http.Request, active bool) {
	id, ok := httpx.PathID(r, "id")
	if !ok {
		httpx.WriteMessage(w, http.StatusNotFound, "", "Cliente no encontrado")
		return
	}
	c, err := h.customers.SetActive(r.Context(), id, active)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, dto.NewAdminCustomerResponse(c))
}

// StockMovements: GET /api/admin/stock-movements?watchId=&type=&from=&to=&page=&limit=
func (h *AdminHandler) StockMovements(w http.ResponseWriter, r *http.Request) {
	v := r.URL.Query()
	var f domain.MovementFilter
	watchID, ok := optionalObjectID(v, "watchId")
	if !ok {
		httpx.WriteMessage(w, http.StatusBadRequest, "", "Reloj inválido")
		return
	}
	f.WatchID = watchID
	if t := v.Get("type"); t != "" {
		if !domain.ValidMoveType(t) {
			httpx.WriteMessage(w, http.StatusBadRequest, "", "Tipo de movimiento inválido")
			return
		}
		f.Type = t
	}
	from, to, msg := parseDateRange(v)
	if msg != "" {
		httpx.WriteMessage(w, http.StatusBadRequest, "", msg)
		return
	}
	f.CreatedFrom, f.CreatedTo = from, to
	page, limit, msg := parsePage(v, 50, "Límite inválido (1 a 100)")
	if msg != "" {
		httpx.WriteMessage(w, http.StatusBadRequest, "", msg)
		return
	}
	items, total, err := h.stock.ListMovements(r.Context(), f, page, limit)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, dto.NewPage(items, page, limit, total))
}

// Upload: POST /api/admin/uploads — form-data "image"; devuelve su URL pública.
func (h *AdminHandler) Upload(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, service.MaxUploadSize)
	if err := r.ParseMultipartForm(service.MaxUploadSize); err != nil {
		httpx.WriteMessage(w, http.StatusBadRequest, "", "La imagen supera los 5 MB permitidos")
		return
	}
	file, _, err := r.FormFile("image")
	if err != nil {
		httpx.WriteMessage(w, http.StatusBadRequest, "", "Adjuntá una imagen en el campo 'image'")
		return
	}
	defer file.Close()
	url, err := h.uploads.SaveImage(r.Context(), file)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, map[string]string{"url": url})
}
