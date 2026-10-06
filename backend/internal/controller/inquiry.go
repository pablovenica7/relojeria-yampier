package controller

import (
	"net/http"

	"relojeria-yampier/internal/dto"
	"relojeria-yampier/internal/httpx"
	"relojeria-yampier/internal/service"
)

// InquiryHandler expone las consultas (alta pública y gestión del Admin).
type InquiryHandler struct{ inquiries *service.InquiryService }

func NewInquiryHandler(inquiries *service.InquiryService) *InquiryHandler {
	return &InquiryHandler{inquiries: inquiries}
}

// Create: POST /api/inquiries (público)
func (h *InquiryHandler) Create(w http.ResponseWriter, r *http.Request) {
	var in dto.InquiryRequest
	if err := httpx.DecodeJSON(w, r, 1<<16, &in); err != nil {
		httpx.WriteMessage(w, http.StatusBadRequest, "", "Datos inválidos")
		return
	}
	if _, err := h.inquiries.Create(r.Context(), in); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, map[string]string{"status": "ok"})
}

// List: GET /api/admin/inquiries?status=&watchId=
func (h *InquiryHandler) List(w http.ResponseWriter, r *http.Request) {
	watchID, ok := optionalObjectID(r.URL.Query(), "watchId")
	if !ok {
		httpx.WriteMessage(w, http.StatusBadRequest, "", "Reloj inválido")
		return
	}
	out, err := h.inquiries.List(r.Context(), r.URL.Query().Get("status"), watchID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, out)
}

// Update: PATCH /api/admin/inquiries/{id} — body {"status"?, "adminNotes"?}
func (h *InquiryHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, ok := httpx.PathID(r, "id")
	if !ok {
		httpx.WriteMessage(w, http.StatusNotFound, "", "Consulta no encontrada")
		return
	}
	var in dto.InquiryUpdateRequest
	if err := httpx.DecodeJSON(w, r, 1<<16, &in); err != nil {
		httpx.WriteMessage(w, http.StatusBadRequest, "", "Datos inválidos")
		return
	}
	q, err := h.inquiries.Update(r.Context(), id, in)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, q)
}

// Delete: DELETE /api/admin/inquiries/{id}
func (h *InquiryHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, ok := httpx.PathID(r, "id")
	if !ok {
		httpx.WriteMessage(w, http.StatusNotFound, "", "Consulta no encontrada")
		return
	}
	if err := h.inquiries.Delete(r.Context(), id); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
