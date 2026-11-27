package main

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"testing/fstest"
)

func prepararServidor(t *testing.T, backendURL string) http.Handler {
	t.Helper()
	sistema := fstest.MapFS{
		"index.html":            &fstest.MapFile{Data: []byte("<!doctype html><html><body>stock</body></html>")},
		"favicon.ico":           &fstest.MapFile{Data: []byte("ico")},
		"assets/app-abc123.js":  &fstest.MapFile{Data: []byte("console.log('ok')")},
		"assets/app-abc123.css": &fstest.MapFile{Data: []byte("body{}")},
	}
	backend, err := validarBackend(backendURL)
	if err != nil {
		t.Fatalf("backend inválido: %v", err)
	}
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	handler, err := nuevoManejador(sistema, backend, logger)
	if err != nil {
		t.Fatalf("no se pudo crear el manejador: %v", err)
	}
	return handler
}

func TestValidarBackend(t *testing.T) {
	if _, err := validarBackend("http://backend:8080"); err != nil {
		t.Fatalf("URL válida rechazada: %v", err)
	}
	for _, invalida := range []string{"", "ftp://backend", "http://user:pass@backend:8080", "http://backend:8080?x=1", "http://backend:8080#a"} {
		if _, err := validarBackend(invalida); err == nil {
			t.Fatalf("se esperaba error para %q", invalida)
		}
	}
}

func TestNuevoManejadorExigeIndex(t *testing.T) {
	backend, _ := url.Parse("http://backend:8080")
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	if _, err := nuevoManejador(fstest.MapFS{}, backend, logger); err == nil {
		t.Fatal("se esperaba error sin index.html")
	}
}

func TestRutasCabecerasYProxy(t *testing.T) {
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Echo-Request-Id", r.Header.Get("X-Request-Id"))
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer api.Close()

	handler := prepararServidor(t, api.URL)

	t.Run("healthz", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"ok"`) {
			t.Fatalf("respuesta inesperada: %d %s", rec.Code, rec.Body.String())
		}
		if rec.Header().Get("X-Content-Type-Options") != "nosniff" || rec.Header().Get("X-Request-Id") == "" {
			t.Fatalf("cabeceras incompletas: %v", rec.Header())
		}
	})

	t.Run("asset-inmutable", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/assets/app-abc123.js", nil)
		req.Header.Set("X-Request-Id", "req-test-1")
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK || !strings.Contains(rec.Header().Get("Cache-Control"), "immutable") {
			t.Fatalf("asset inesperado: %d %v", rec.Code, rec.Header())
		}
		if rec.Header().Get("X-Request-Id") != "req-test-1" {
			t.Fatal("no se conservó X-Request-Id")
		}
	})

	t.Run("asset-inexistente-y-traversal", func(t *testing.T) {
		for _, ruta := range []string{"/assets/no-existe.js", "/assets/", "/assets"} {
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, ruta, nil))
			if rec.Code != http.StatusNotFound {
				t.Fatalf("ruta %s devolvió %d", ruta, rec.Code)
			}
		}
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/assets/../etc/passwd", nil))
		if strings.Contains(rec.Body.String(), "root:") {
			t.Fatal("traversal permitido")
		}
	})

	t.Run("respuesta-registrada-unwrap", func(t *testing.T) {
		base := httptest.NewRecorder()
		env := &respuestaRegistrada{ResponseWriter: base}
		if env.Unwrap() != base {
			t.Fatal("Unwrap no devolvió el ResponseWriter base")
		}
		_, _ = env.Write([]byte("ok"))
		if env.estado != http.StatusOK {
			t.Fatalf("estado implícito inesperado: %d", env.estado)
		}
	})

	t.Run("archivo-estatico-y-fallback-spa", func(t *testing.T) {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/favicon.ico", nil))
		if rec.Code != http.StatusOK || rec.Body.String() != "ico" {
			t.Fatalf("favicon inesperado: %d %s", rec.Code, rec.Body.String())
		}
		spa := httptest.NewRecorder()
		handler.ServeHTTP(spa, httptest.NewRequest(http.MethodGet, "/movimientos/detalle", nil))
		if spa.Code != http.StatusOK || !strings.Contains(spa.Body.String(), "stock") || spa.Header().Get("Cache-Control") != "no-cache" {
			t.Fatalf("fallback SPA inesperado: %d %v %s", spa.Code, spa.Header(), spa.Body.String())
		}
	})

	t.Run("metodo-no-permitido", func(t *testing.T) {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/movimientos", nil))
		if rec.Code != http.StatusMethodNotAllowed || rec.Header().Get("Allow") != "GET, HEAD" {
			t.Fatalf("estado inesperado: %d %v", rec.Code, rec.Header())
		}
	})

	t.Run("proxy-api-ok-y-caido", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/v1/existencias", nil)
		req.Header.Set("X-Request-Id", "req-proxy-1")
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK || rec.Header().Get("X-Echo-Request-Id") != "req-proxy-1" {
			t.Fatalf("proxy falló: %d %v", rec.Code, rec.Header())
		}

		api.Close()
		caido := httptest.NewRecorder()
		handler.ServeHTTP(caido, httptest.NewRequest(http.MethodGet, "/api/v1/existencias", nil))
		if caido.Code != http.StatusBadGateway || caido.Header().Get("Content-Type") != "application/problem+json" {
			t.Fatalf("error de proxy inesperado: %d %v", caido.Code, caido.Header())
		}
	})
}
