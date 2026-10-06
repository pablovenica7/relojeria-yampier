package service

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"relojeria-yampier/internal/domain"
)

// MaxUploadSize es el tamaño máximo de una imagen subida.
const MaxUploadSize = 5 << 20 // 5 MB

// Extensión de salida según el MIME REAL detectado por sniffing, no según el
// nombre del archivo original (que el cliente puede falsear).
var allowedImageExt = map[string]string{
	"image/jpeg": ".jpg",
	"image/png":  ".png",
	"image/webp": ".webp",
}

// UploadService guarda imágenes del catálogo en disco (UPLOADS_DIR).
type UploadService struct {
	dir string
	now Clock
}

func NewUploadService(dir string, now Clock) *UploadService {
	return &UploadService{dir: dir, now: clockOrNow(now)}
}

// Dir es la carpeta servida públicamente en /uploads/.
func (s *UploadService) Dir() string { return s.dir }

// SaveImage valida el contenido real y guarda el archivo con un nombre
// generado por el servidor (evita path traversal, colisiones y nombres
// peligrosos). Devuelve la URL pública (/uploads/...).
func (s *UploadService) SaveImage(file io.ReadSeeker) (string, error) {
	head := make([]byte, 512)
	n, err := file.Read(head)
	if err != nil && err != io.EOF {
		return "", domain.Validation("No se pudo leer la imagen")
	}
	ext, ok := allowedImageExt[http.DetectContentType(head[:n])]
	if !ok {
		return "", domain.Validation("Formato no permitido (usá jpg, png o webp)")
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return "", fmt.Errorf("no se pudo procesar la imagen: %w", err)
	}
	if err := os.MkdirAll(s.dir, 0o755); err != nil {
		return "", fmt.Errorf("no se pudo preparar el almacenamiento: %w", err)
	}
	filename := fmt.Sprintf("%d%s", s.now().UnixNano(), ext)
	dst, err := os.Create(filepath.Join(s.dir, filename))
	if err != nil {
		return "", fmt.Errorf("no se pudo guardar la imagen: %w", err)
	}
	defer dst.Close()
	if _, err := io.Copy(dst, file); err != nil {
		return "", fmt.Errorf("no se pudo guardar la imagen: %w", err)
	}
	return "/uploads/" + filename, nil
}
