package service

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"

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

// ImageStore persiste las imágenes (GridFS en producción, fake en tests).
type ImageStore interface {
	Save(ctx context.Context, filename, contentType string, content io.Reader) error
	Open(ctx context.Context, filename string) (*domain.Image, error)
	Exists(ctx context.Context, filename string) (bool, error)
}

// UploadService guarda y sirve las imágenes del catálogo. Se guardan en
// MongoDB (no en el disco del contenedor) para que no se pierdan en cada
// redeploy del hosting y queden incluidas en los backups de la base.
type UploadService struct {
	store ImageStore
	now   Clock
}

func NewUploadService(store ImageStore, now Clock) *UploadService {
	return &UploadService{store: store, now: clockOrNow(now)}
}

var errImageNotFound = domain.NotFound("", "Imagen no encontrada")

// SaveImage valida el contenido real y guarda el archivo con un nombre
// generado por el servidor (evita path traversal, colisiones y nombres
// peligrosos). Devuelve la URL pública (/uploads/...).
func (s *UploadService) SaveImage(ctx context.Context, file io.ReadSeeker) (string, error) {
	head := make([]byte, 512)
	n, err := file.Read(head)
	if err != nil && err != io.EOF {
		return "", domain.Validation("No se pudo leer la imagen")
	}
	contentType := http.DetectContentType(head[:n])
	ext, ok := allowedImageExt[contentType]
	if !ok {
		return "", domain.Validation("Formato no permitido (usá jpg, png o webp)")
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return "", fmt.Errorf("no se pudo procesar la imagen: %w", err)
	}
	filename := fmt.Sprintf("%d%s", s.now().UnixNano(), ext)
	if err := s.store.Save(ctx, filename, contentType, file); err != nil {
		return "", fmt.Errorf("no se pudo guardar la imagen: %w", err)
	}
	return "/uploads/" + filename, nil
}

// OpenImage devuelve una imagen para servirla en /uploads/{name}.
func (s *UploadService) OpenImage(ctx context.Context, name string) (*domain.Image, error) {
	if !domain.ValidImageName(name) {
		return nil, errImageNotFound
	}
	img, err := s.store.Open(ctx, name)
	if isNotFound(err) {
		return nil, errImageNotFound
	}
	if err != nil {
		return nil, err
	}
	if img.ContentType == "" {
		img.ContentType = contentTypeForName(name)
	}
	return img, nil
}

func contentTypeForName(name string) string {
	ext := strings.ToLower(filepath.Ext(name))
	for ct, e := range allowedImageExt {
		if e == ext {
			return ct
		}
	}
	return "application/octet-stream"
}

// ImportLegacyDir copia a la base las imágenes que versiones anteriores
// guardaban en disco (UPLOADS_DIR), para que las URLs /uploads/... ya
// guardadas en los relojes sigan funcionando. Es idempotente: saltea las que
// ya existen. Si la carpeta no existe (caso normal en el hosting), no hace nada.
func (s *UploadService) ImportLegacyDir(ctx context.Context, dir string, log *slog.Logger) {
	log = loggerOrDefault(log)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	imported := 0
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !domain.ValidImageName(name) {
			continue
		}
		exists, err := s.store.Exists(ctx, name)
		if err != nil {
			log.Warn("no se pudieron migrar las imágenes en disco", "error", err)
			return
		}
		if exists {
			continue
		}
		if err := s.importFile(ctx, filepath.Join(dir, name), name); err != nil {
			log.Warn("no se pudo migrar una imagen en disco", "file", name, "error", err)
			continue
		}
		imported++
	}
	if imported > 0 {
		log.Info("imágenes en disco migradas a MongoDB", "count", imported)
	}
}

func (s *UploadService) importFile(ctx context.Context, path, name string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return s.store.Save(ctx, name, contentTypeForName(name), f)
}
