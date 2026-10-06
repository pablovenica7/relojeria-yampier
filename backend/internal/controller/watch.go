package controller

import (
	"net/http"

	"relojeria-yampier/internal/dto"
	"relojeria-yampier/internal/httpx"
	"relojeria-yampier/internal/service"
)

// WatchHandler expone el catálogo (público y Admin).
type WatchHandler struct{ watches *service.WatchService }

func NewWatchHandler(watches *service.WatchService) *WatchHandler {
	return &WatchHandler{watches: watches}
}

// List: GET /api/watches?search=&brand=&gender=&availability=&sort=&page=&limit=
func (h *WatchHandler) List(w http.ResponseWriter, r *http.Request) {
	f, msg := parseCatalogQuery(r.URL.Query(), false)
	if msg != "" {
		httpx.WriteMessage(w, http.StatusBadRequest, "", msg)
		return
	}
	out, err := h.watches.ListPublic(r.Context(), f)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, out)
}

// Get: GET /api/watches/{id} — un reloj archivado responde 404.
func (h *WatchHandler) Get(w http.ResponseWriter, r *http.Request) { h.get(w, r, true) }

// AdminGet: GET /api/admin/watches/{id} — incluye archivados.
func (h *WatchHandler) AdminGet(w http.ResponseWriter, r *http.Request) { h.get(w, r, false) }

func (h *WatchHandler) get(w http.ResponseWriter, r *http.Request, publicOnly bool) {
	id, ok := httpx.PathID(r, "id")
	if !ok {
		httpx.WriteMessage(w, http.StatusNotFound, "", "Reloj no encontrado")
		return
	}
	watch, err := h.watches.Get(r.Context(), id, publicOnly)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, watch)
}

// AdminList: GET /api/admin/watches — permite status=active|archived|all y agrega contadores.
func (h *WatchHandler) AdminList(w http.ResponseWriter, r *http.Request) {
	f, msg := parseCatalogQuery(r.URL.Query(), true)
	if msg != "" {
		httpx.WriteMessage(w, http.StatusBadRequest, "", msg)
		return
	}
	out, err := h.watches.ListAdmin(r.Context(), f)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, out)
}

func decodeWatch(w http.ResponseWriter, r *http.Request) (dto.WatchRequest, bool) {
	var in dto.WatchRequest
	if err := httpx.DecodeJSON(w, r, 1<<20, &in); err != nil {
		httpx.WriteMessage(w, http.StatusBadRequest, "", "Datos inválidos (revisá que precio y stock sean números enteros)")
		return in, false
	}
	return in, true
}

// Create: POST /api/admin/watches
func (h *WatchHandler) Create(w http.ResponseWriter, r *http.Request) {
	in, ok := decodeWatch(w, r)
	if !ok {
		return
	}
	watch, err := h.watches.Create(r.Context(), in, httpx.AdminEmail(r.Context()))
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, watch)
}

// Update: PUT /api/admin/watches/{id}
func (h *WatchHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, ok := httpx.PathID(r, "id")
	if !ok {
		httpx.WriteMessage(w, http.StatusNotFound, "", "Reloj no encontrado")
		return
	}
	in, ok := decodeWatch(w, r)
	if !ok {
		return
	}
	watch, err := h.watches.Update(r.Context(), id, in, httpx.AdminEmail(r.Context()))
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, watch)
}

// AdjustStock: PATCH /api/admin/watches/{id}/stock — body {"delta", "type"?, "reason"?}
func (h *WatchHandler) AdjustStock(w http.ResponseWriter, r *http.Request) {
	id, ok := httpx.PathID(r, "id")
	if !ok {
		httpx.WriteMessage(w, http.StatusNotFound, "", "Reloj no encontrado")
		return
	}
	var in dto.StockAdjustRequest
	if err := httpx.DecodeJSON(w, r, 1<<12, &in); err != nil {
		httpx.WriteMessage(w, http.StatusBadRequest, "", "Indicá un ajuste de stock entero distinto de cero")
		return
	}
	watch, err := h.watches.AdjustStock(r.Context(), id, in.Delta, in.Type, in.Reason, httpx.AdminEmail(r.Context()))
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, watch)
}

// Archive: PATCH /api/admin/watches/{id}/archive[?confirm=true]
func (h *WatchHandler) Archive(w http.ResponseWriter, r *http.Request) {
	id, ok := httpx.PathID(r, "id")
	if !ok {
		httpx.WriteMessage(w, http.StatusNotFound, "", "Reloj no encontrado")
		return
	}
	watch, err := h.watches.Archive(r.Context(), id, r.URL.Query().Get("confirm") == "true")
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, watch)
}

// Restore: PATCH /api/admin/watches/{id}/restore
func (h *WatchHandler) Restore(w http.ResponseWriter, r *http.Request) {
	id, ok := httpx.PathID(r, "id")
	if !ok {
		httpx.WriteMessage(w, http.StatusNotFound, "", "Reloj no encontrado")
		return
	}
	watch, err := h.watches.Restore(r.Context(), id)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, watch)
}

// Delete: DELETE /api/admin/watches/{id} — excepcional (solo archivados sin reservas).
func (h *WatchHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, ok := httpx.PathID(r, "id")
	if !ok {
		httpx.WriteMessage(w, http.StatusNotFound, "", "Reloj no encontrado")
		return
	}
	if err := h.watches.Delete(r.Context(), id); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
