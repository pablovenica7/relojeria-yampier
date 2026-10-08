package service

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"relojeria-yampier/internal/domain"
)

type fakeImageStore struct {
	mu    sync.Mutex
	files map[string][]byte
	types map[string]string
}

func newFakeImageStore() *fakeImageStore {
	return &fakeImageStore{files: map[string][]byte{}, types: map[string]string{}}
}

func (f *fakeImageStore) Save(_ context.Context, name, ct string, r io.Reader) error {
	b, err := io.ReadAll(r)
	if err != nil {
		return err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.files[name], f.types[name] = b, ct
	return nil
}

func (f *fakeImageStore) Open(_ context.Context, name string) (*domain.Image, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	b, ok := f.files[name]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return &domain.Image{Content: io.NopCloser(bytes.NewReader(b)), Size: int64(len(b)), ContentType: f.types[name]}, nil
}

func (f *fakeImageStore) Exists(_ context.Context, name string) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	_, ok := f.files[name]
	return ok, nil
}

var tinyPNG, _ = base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8BQDwAEhQGAhKmMIQAAAABJRU5ErkJggg==")

func TestSaveImageStoresSniffedTypeAndServesIt(t *testing.T) {
	store := newFakeImageStore()
	svc := NewUploadService(store, func() time.Time { return time.Unix(0, 1700) })
	url, err := svc.SaveImage(context.Background(), bytes.NewReader(tinyPNG))
	if err != nil || url != "/uploads/1700.png" {
		t.Fatalf("url=%q err=%v", url, err)
	}
	img, err := svc.OpenImage(context.Background(), "1700.png")
	if err != nil {
		t.Fatal(err)
	}
	defer img.Content.Close()
	got, _ := io.ReadAll(img.Content)
	if img.ContentType != "image/png" || !bytes.Equal(got, tinyPNG) {
		t.Errorf("contenido o tipo incorrecto: %q", img.ContentType)
	}
}

func TestSaveImageRejectsNonImages(t *testing.T) {
	svc := NewUploadService(newFakeImageStore(), nil)
	_, err := svc.SaveImage(context.Background(), strings.NewReader("<script>alert(1)</script>"))
	if !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("se esperaba error de validación, obtuvo %v", err)
	}
}

func TestOpenImageRejectsUnexpectedNames(t *testing.T) {
	svc := NewUploadService(newFakeImageStore(), nil)
	for _, name := range []string{"../.env", "a.png", "123.svg", "123.png/x", "", "999.png"} {
		if _, err := svc.OpenImage(context.Background(), name); !errors.Is(err, domain.ErrNotFound) {
			t.Errorf("%q: se esperaba 404, obtuvo %v", name, err)
		}
	}
}

func TestImportLegacyDirIsIdempotent(t *testing.T) {
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "111.png"), tinyPNG, 0o644)
	_ = os.WriteFile(filepath.Join(dir, "notas.txt"), []byte("x"), 0o644)
	store := newFakeImageStore()
	svc := NewUploadService(store, nil)
	svc.ImportLegacyDir(context.Background(), dir, nil)
	svc.ImportLegacyDir(context.Background(), dir, nil)
	svc.ImportLegacyDir(context.Background(), filepath.Join(dir, "no-existe"), nil)
	if len(store.files) != 1 || store.types["111.png"] != "image/png" {
		t.Fatalf("se esperaba 1 imagen migrada, hay %v", store.types)
	}
}
