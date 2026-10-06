package controller

import (
	"log/slog"
	"net/http"

	"relojeria-yampier/internal/dto"
	"relojeria-yampier/internal/httpx"
	"relojeria-yampier/internal/service"
)

// AccountHandler expone las cuentas de clientes (/api/auth/...) y el login
// del Admin. Son autenticaciones separadas: otro rol en el token.
type AccountHandler struct {
	customers *service.CustomerService
	resets    *service.PasswordResetService
	admins    *service.AdminAuthService
	log       *slog.Logger
}

func NewAccountHandler(customers *service.CustomerService, resets *service.PasswordResetService, admins *service.AdminAuthService, log *slog.Logger) *AccountHandler {
	return &AccountHandler{customers: customers, resets: resets, admins: admins, log: log}
}

// AdminLogin: POST /api/admin/login
func (h *AccountHandler) AdminLogin(w http.ResponseWriter, r *http.Request) {
	var in dto.LoginRequest
	if err := httpx.DecodeJSON(w, r, 1<<16, &in); err != nil {
		httpx.WriteMessage(w, http.StatusBadRequest, "", "Datos inválidos")
		return
	}
	token, email, err := h.admins.Login(r.Context(), in.Email, in.Password)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]string{"token": token, "email": email})
}

// Register: POST /api/auth/register
func (h *AccountHandler) Register(w http.ResponseWriter, r *http.Request) {
	var in dto.CustomerRequest
	if err := httpx.DecodeJSON(w, r, 1<<16, &in); err != nil {
		httpx.WriteMessage(w, http.StatusBadRequest, "", "Datos inválidos")
		return
	}
	token, c, err := h.customers.Register(r.Context(), in)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, dto.AuthResponse{Token: token, Customer: dto.NewCustomerResponse(c)})
}

// Login: POST /api/auth/login (clientes)
func (h *AccountHandler) Login(w http.ResponseWriter, r *http.Request) {
	var in dto.LoginRequest
	if err := httpx.DecodeJSON(w, r, 1<<12, &in); err != nil {
		httpx.WriteMessage(w, http.StatusBadRequest, "", "Datos inválidos")
		return
	}
	token, c, err := h.customers.Login(r.Context(), in.Email, in.Password)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, dto.AuthResponse{Token: token, Customer: dto.NewCustomerResponse(c)})
}

// Me: GET /api/auth/me — siempre el perfil del dueño del token.
func (h *AccountHandler) Me(w http.ResponseWriter, r *http.Request) {
	httpx.WriteJSON(w, http.StatusOK, dto.NewCustomerResponse(httpx.Customer(r.Context())))
}

// UpdateMe: PUT /api/auth/me
func (h *AccountHandler) UpdateMe(w http.ResponseWriter, r *http.Request) {
	var in dto.CustomerRequest
	if err := httpx.DecodeJSON(w, r, 1<<16, &in); err != nil {
		httpx.WriteMessage(w, http.StatusBadRequest, "", "Datos inválidos")
		return
	}
	c, err := h.customers.UpdateProfile(r.Context(), httpx.Customer(r.Context()), in)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, dto.NewCustomerResponse(c))
}

// ChangePassword: POST /api/auth/me/password — devuelve un token nuevo.
func (h *AccountHandler) ChangePassword(w http.ResponseWriter, r *http.Request) {
	var in dto.ChangePasswordRequest
	if err := httpx.DecodeJSON(w, r, 1<<12, &in); err != nil {
		httpx.WriteMessage(w, http.StatusBadRequest, "", "Datos inválidos")
		return
	}
	token, err := h.customers.ChangePassword(r.Context(), httpx.Customer(r.Context()), in.CurrentPassword, in.NewPassword)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok", "token": token})
}

// ForgotPassword: POST /api/auth/password/forgot — respuesta SIEMPRE igual.
func (h *AccountHandler) ForgotPassword(w http.ResponseWriter, r *http.Request) {
	var in dto.ForgotPasswordRequest
	if err := httpx.DecodeJSON(w, r, 1<<10, &in); err != nil {
		httpx.WriteMessage(w, http.StatusBadRequest, "", "Datos inválidos")
		return
	}
	if _, err := h.resets.Request(r.Context(), in.Email); err != nil {
		// Se registra sin el email ni el token; el cliente recibe lo mismo.
		h.log.Error("error en recuperación de contraseña", "request_id", httpx.RequestID(r.Context()), "error", err)
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]string{
		"message": "Si el email corresponde a una cuenta, vas a recibir un enlace para restablecer la contraseña.",
	})
}

// ResetPassword: POST /api/auth/password/reset — body {token, newPassword}
func (h *AccountHandler) ResetPassword(w http.ResponseWriter, r *http.Request) {
	var in dto.ResetPasswordRequest
	if err := httpx.DecodeJSON(w, r, 1<<10, &in); err != nil {
		httpx.WriteMessage(w, http.StatusBadRequest, "", "Datos inválidos")
		return
	}
	if err := h.resets.Reset(r.Context(), in.Token, in.NewPassword); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
