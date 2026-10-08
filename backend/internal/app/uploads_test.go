package app

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

var pngBytes, _ = base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8BQDwAEhQGAhKmMIQAAAABJRU5ErkJggg==")

func (ta *testApp) get(path string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	ta.h.ServeHTTP(rec, httptest.NewRequest("GET", path, nil))
	return rec
}

// Las imágenes quedan en MongoDB (GridFS): sobreviven a un redeploy del
// contenedor y las sirve cualquier instancia del backend.
func TestUploadedImagesLiveInMongo(t *testing.T) {
	ta := newTestApp(t)
	admin := ta.adminToken()

	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	fw, _ := mw.CreateFormFile("image", "foto.png")
	_, _ = fw.Write(pngBytes)
	_ = mw.Close()
	req := httptest.NewRequest("POST", "/api/admin/uploads", &body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+admin)
	rec := httptest.NewRecorder()
	ta.h.ServeHTTP(rec, req)
	var res struct{ URL string }
	_ = json.Unmarshal(rec.Body.Bytes(), &res)
	if rec.Code != http.StatusCreated || res.URL == "" {
		t.Fatalf("upload: %d %s", rec.Code, rec.Body.String())
	}

	// Una instancia NUEVA de la app (como tras un redeploy) sirve la imagen.
	fresh := New(ta.app.cfg, ta.db, ta.app.log).Handler()
	got := httptest.NewRecorder()
	fresh.ServeHTTP(got, httptest.NewRequest("GET", res.URL, nil))
	if got.Code != http.StatusOK || !bytes.Equal(got.Body.Bytes(), pngBytes) {
		t.Fatalf("GET %s: %d", res.URL, got.Code)
	}
	if ct := got.Header().Get("Content-Type"); ct != "image/png" {
		t.Errorf("Content-Type = %q", ct)
	}
	if cc := got.Header().Get("Cache-Control"); cc == "" {
		t.Error("falta Cache-Control")
	}

	if code := ta.get("/uploads/123.png").Code; code != http.StatusNotFound {
		t.Errorf("imagen inexistente: %d", code)
	}
	if code := ta.get("/uploads/..%2f.env").Code; code != http.StatusNotFound {
		t.Errorf("nombre inválido: %d", code)
	}
}

func TestLegacyDiskUploadsAreMigrated(t *testing.T) {
	ta := newTestApp(t)
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "1700000000000000000.png"), pngBytes, 0o644); err != nil {
		t.Fatal(err)
	}
	ta.app.Services.Uploads.ImportLegacyDir(context.Background(), dir, ta.app.log)
	ta.app.Services.Uploads.ImportLegacyDir(context.Background(), dir, ta.app.log) // idempotente
	rec := ta.get("/uploads/1700000000000000000.png")
	if rec.Code != http.StatusOK || !bytes.Equal(rec.Body.Bytes(), pngBytes) {
		t.Fatalf("imagen migrada no disponible: %d", rec.Code)
	}
	n, _ := ta.col("uploads.files").CountDocuments(context.Background(), map[string]any{})
	if n != 1 {
		t.Errorf("se esperaba 1 archivo en GridFS, hay %d", n)
	}
}
