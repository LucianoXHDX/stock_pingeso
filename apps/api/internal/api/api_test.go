package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func pedir(mux *http.ServeMux, ruta string) *httptest.ResponseRecorder {
	respuesta := httptest.NewRecorder()
	mux.ServeHTTP(respuesta, httptest.NewRequest(http.MethodGet, ruta, nil))
	return respuesta
}

func TestHealthzResponde200(t *testing.T) {
	mux := http.NewServeMux()
	NuevaAPI(mux, Opciones{Version: "abc1234", Entorno: "test"})

	respuesta := pedir(mux, "/healthz")
	if respuesta.Code != http.StatusOK {
		t.Fatalf("estado = %d, se esperaba 200", respuesta.Code)
	}

	var cuerpo EstadoVivo
	if err := json.Unmarshal(respuesta.Body.Bytes(), &cuerpo); err != nil {
		t.Fatalf("el cuerpo no es JSON: %v", err)
	}
	if cuerpo.Estado != "ok" || cuerpo.Version != "abc1234" {
		t.Fatalf("cuerpo inesperado: %+v", cuerpo)
	}
}

func TestDocsSoloFueraDeProd(t *testing.T) {
	dev := http.NewServeMux()
	NuevaAPI(dev, Opciones{Entorno: "dev"})
	if codigo := pedir(dev, "/api/docs").Code; codigo != http.StatusOK {
		t.Fatalf("en dev /api/docs = %d, se esperaba 200", codigo)
	}

	prod := http.NewServeMux()
	NuevaAPI(prod, Opciones{Entorno: "prod"})
	if codigo := pedir(prod, "/api/docs").Code; codigo != http.StatusNotFound {
		t.Fatalf("en prod /api/docs = %d, se esperaba 404", codigo)
	}
}

func TestGenerarContratoIncluyeSalud(t *testing.T) {
	contenido, err := GenerarContrato()
	if err != nil {
		t.Fatalf("no se esperaba error: %v", err)
	}

	texto := string(contenido)
	if !strings.Contains(texto, "/healthz") || !strings.Contains(texto, VersionContrato) {
		t.Fatalf("el contrato no tiene lo esperado:\n%s", texto)
	}
}
