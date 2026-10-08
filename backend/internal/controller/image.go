package controller

import (
	"io"
	"net/http"
	"strconv"

	"relojeria-yampier/internal/httpx"
	"relojeria-yampier/internal/service"
)

// ImageHandler sirve las imágenes subidas desde el Admin (públicas).
type ImageHandler struct{ uploads *service.UploadService }

func NewImageHandler(uploads *service.UploadService) *ImageHandler {
	return &ImageHandler{uploads: uploads}
}

// Serve: GET /uploads/{name}. El nombre es único por imagen (nunca se pisa),
// así que el navegador y la CDN pueden cachearla para siempre.
func (h *ImageHandler) Serve(w http.ResponseWriter, r *http.Request) {
	img, err := h.uploads.OpenImage(r.Context(), r.PathValue("name"))
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	defer img.Content.Close()
	w.Header().Set("Content-Type", img.ContentType)
	w.Header().Set("Content-Length", strconv.FormatInt(img.Size, 10))
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	if !img.UploadedAt.IsZero() {
		w.Header().Set("Last-Modified", img.UploadedAt.UTC().Format(http.TimeFormat))
	}
	w.WriteHeader(http.StatusOK)
	if r.Method == http.MethodHead {
		return
	}
	_, _ = io.Copy(w, img.Content)
}
