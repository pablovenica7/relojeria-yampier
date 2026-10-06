package controller

import (
	"net/http"

	"relojeria-yampier/internal/domain"
	"relojeria-yampier/internal/dto"
	"relojeria-yampier/internal/httpx"
	"relojeria-yampier/internal/service"
)

// ReservationHandler expone reservas: solicitudes de clientes y gestión del Admin.
type ReservationHandler struct{ reservations *service.ReservationService }

func NewReservationHandler(reservations *service.ReservationService) *ReservationHandler {
	return &ReservationHandler{reservations: reservations}
}

// ---------- Admin ----------

// AdminList: GET /api/admin/reservations?status=&watchId=&from=&to=&page=&limit=
func (h *ReservationHandler) AdminList(w http.ResponseWriter, r *http.Request) {
	v := r.URL.Query()
	var f domain.ReservationFilter
	if s := v.Get("status"); s != "" {
		if !domain.ValidReservationStatus(s) {
			httpx.WriteMessage(w, http.StatusBadRequest, "", "Estado inválido")
			return
		}
		f.Status = s
	}
	watchID, ok := optionalObjectID(v, "watchId")
	if !ok {
		httpx.WriteMessage(w, http.StatusBadRequest, "", "Reloj inválido")
		return
	}
	f.WatchID = watchID
	from, to, msg := parseDateRange(v)
	if msg != "" {
		httpx.WriteMessage(w, http.StatusBadRequest, "", msg)
		return
	}
	f.CreatedFrom, f.CreatedTo = from, to
	page, limit, msg := parsePage(v, 50, "Límite inválido")
	if msg != "" {
		httpx.WriteMessage(w, http.StatusBadRequest, "", msg)
		return
	}
	out, err := h.reservations.ListForAdmin(r.Context(), f, page, limit)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, out)
}

// AdminCreate: POST /api/admin/reservations
func (h *ReservationHandler) AdminCreate(w http.ResponseWriter, r *http.Request) {
	var in dto.AdminReservationRequest
	if err := httpx.DecodeJSON(w, r, 1<<16, &in); err != nil {
		httpx.WriteMessage(w, http.StatusBadRequest, "", "Datos inválidos (montos enteros y fechas en formato ISO)")
		return
	}
	res, err := h.reservations.CreateByAdmin(r.Context(), in, httpx.AdminEmail(r.Context()))
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, res)
}

// AdminChangeStatus: PATCH /api/admin/reservations/{id}/status — body {"status", "depositAmount"?, "depositMethod"?}
func (h *ReservationHandler) AdminChangeStatus(w http.ResponseWriter, r *http.Request) {
	id, ok := httpx.PathID(r, "id")
	if !ok {
		httpx.WriteMessage(w, http.StatusNotFound, "", "Reserva no encontrada")
		return
	}
	var in dto.ReservationStatusRequest
	if err := httpx.DecodeJSON(w, r, 1<<12, &in); err != nil {
		httpx.WriteMessage(w, http.StatusBadRequest, "", "Datos inválidos")
		return
	}
	res, err := h.reservations.ChangeStatus(r.Context(), id, in.Status, in.DepositAmount, in.DepositMethod, httpx.AdminEmail(r.Context()))
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, res)
}

// AdminUpdateDetails: PATCH /api/admin/reservations/{id} — notas, seña acordada y vencimiento.
func (h *ReservationHandler) AdminUpdateDetails(w http.ResponseWriter, r *http.Request) {
	id, ok := httpx.PathID(r, "id")
	if !ok {
		httpx.WriteMessage(w, http.StatusNotFound, "", "Reserva no encontrada")
		return
	}
	var in dto.ReservationDetailsRequest
	if err := httpx.DecodeJSON(w, r, 1<<16, &in); err != nil {
		httpx.WriteMessage(w, http.StatusBadRequest, "", "Datos inválidos")
		return
	}
	res, err := h.reservations.UpdateDetails(r.Context(), id, in)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, res)
}

// ---------- Cliente (el cliente sale SIEMPRE del token) ----------

// CustomerCreate: POST /api/reservations — solicitud (queda pendiente).
func (h *ReservationHandler) CustomerCreate(w http.ResponseWriter, r *http.Request) {
	var in dto.CustomerReservationRequest
	if err := httpx.DecodeJSON(w, r, 1<<12, &in); err != nil {
		httpx.WriteMessage(w, http.StatusBadRequest, "", "Datos inválidos")
		return
	}
	res, err := h.reservations.RequestByCustomer(r.Context(), httpx.Customer(r.Context()), in)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, dto.NewCustomerReservationResponse(res))
}

// CustomerList: GET /api/reservations — solo las del cliente autenticado.
func (h *ReservationHandler) CustomerList(w http.ResponseWriter, r *http.Request) {
	out, err := h.reservations.ListForCustomer(r.Context(), httpx.Customer(r.Context()).ID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, out)
}

// CustomerGet: GET /api/reservations/{id} — ajena o inexistente → 404.
func (h *ReservationHandler) CustomerGet(w http.ResponseWriter, r *http.Request) {
	id, ok := httpx.PathID(r, "id")
	if !ok {
		httpx.WriteMessage(w, http.StatusNotFound, "", "Reserva no encontrada")
		return
	}
	res, err := h.reservations.GetForCustomer(r.Context(), httpx.Customer(r.Context()).ID, id)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, dto.NewCustomerReservationResponse(res))
}

// CustomerCancel: POST /api/reservations/{id}/cancel — solo pendientes propias.
func (h *ReservationHandler) CustomerCancel(w http.ResponseWriter, r *http.Request) {
	id, ok := httpx.PathID(r, "id")
	if !ok {
		httpx.WriteMessage(w, http.StatusNotFound, "", "Reserva no encontrada")
		return
	}
	res, err := h.reservations.CancelByCustomer(r.Context(), httpx.Customer(r.Context()).ID, id)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, dto.NewCustomerReservationResponse(res))
}
