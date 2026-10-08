package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCORSAllowsOnlyConfiguredOrigins(t *testing.T) {
	h := CORS("https://relojeriayampier.com, https://www.relojeriayampier.com/")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	cases := []struct {
		origin, want string
	}{
		{"https://relojeriayampier.com", "https://relojeriayampier.com"},
		{"https://www.relojeriayampier.com", "https://www.relojeriayampier.com"},
		{"https://otro-sitio.com", ""},
		{"", ""},
	}
	for _, c := range cases {
		r := httptest.NewRequest("GET", "/api/watches", nil)
		if c.origin != "" {
			r.Header.Set("Origin", c.origin)
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, r)
		if got := rec.Header().Get("Access-Control-Allow-Origin"); got != c.want {
			t.Errorf("origin %q: Allow-Origin = %q, se esperaba %q", c.origin, got, c.want)
		}
		if got := rec.Header().Get("Access-Control-Allow-Origin"); got == "*" {
			t.Errorf("nunca se debe responder con *")
		}
	}

	pre := httptest.NewRequest("OPTIONS", "/api/watches", nil)
	pre.Header.Set("Origin", "https://relojeriayampier.com")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, pre)
	if rec.Code != http.StatusNoContent || rec.Header().Get("Access-Control-Allow-Headers") == "" {
		t.Errorf("preflight: status %d, headers %v", rec.Code, rec.Header())
	}
}
