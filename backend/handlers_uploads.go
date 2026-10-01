package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

const maxUploadSize = 5 << 20 // 5 MB

// Extensión de salida permitida según el MIME real detectado por sniffing,
// no según lo que diga el nombre del archivo original (que no es confiable).
var allowedMIMEExt = map[string]string{
	"image/jpeg": ".jpg",
	"image/png":  ".png",
	"image/webp": ".webp",
}

func uploadsDir() string {
	d := os.Getenv("UPLOADS_DIR")
	if d == "" {
		d = "./uploads"
	}
	return d
}

// POST /api/admin/uploads  (protegido) — recibe un form-data "image" y devuelve su URL pública.
func postUpload(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxUploadSize)
	if err := r.ParseMultipartForm(maxUploadSize); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "La imagen supera los 5 MB permitidos"})
		return
	}
	file, _, err := r.FormFile("image")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Adjuntá una imagen en el campo 'image'"})
		return
	}
	defer file.Close()

	// Sniffing real del contenido: no confiamos en la extensión ni en el
	// nombre del archivo original, que el cliente puede falsear fácilmente.
	head := make([]byte, 512)
	n, err := file.Read(head)
	if err != nil && err != io.EOF {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "No se pudo leer la imagen"})
		return
	}
	contentType := http.DetectContentType(head[:n])
	ext, ok := allowedMIMEExt[contentType]
	if !ok {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Formato no permitido (usá jpg, png o webp)"})
		return
	}
	// Volvemos al inicio del archivo para copiarlo completo a disco.
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "No se pudo procesar la imagen"})
		return
	}

	if err := os.MkdirAll(uploadsDir(), 0o755); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "No se pudo preparar el almacenamiento"})
		return
	}

	// El nombre lo genera siempre el servidor (timestamp + extensión derivada
	// del contenido real). Nunca se usa el nombre original del archivo: evita
	// path traversal, colisiones y nombres con caracteres peligrosos.
	filename := fmt.Sprintf("%d%s", time.Now().UnixNano(), ext)
	dstPath := filepath.Join(uploadsDir(), filename)
	dst, err := os.Create(dstPath)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "No se pudo guardar la imagen"})
		return
	}
	defer dst.Close()

	if _, err := io.Copy(dst, file); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "No se pudo guardar la imagen"})
		return
	}

	writeJSON(w, http.StatusCreated, map[string]string{"url": "/uploads/" + filename})
}
