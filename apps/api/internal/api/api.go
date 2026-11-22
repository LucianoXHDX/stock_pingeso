// Package api contiene los handlers Huma y los structs del contrato del servicio.
package api

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humago"
)

// VersionContrato es la versión del contrato OpenAPI. Cambia solo cuando cambia el contrato,
// nunca con cada compilación, para que pnpm codegen sea reproducible.
const VersionContrato = "1.0.0"

// Opciones configura la construcción de la API.
type Opciones struct {
	// Version del binario, normalmente el SHA corto del commit.
	Version string

	// Entorno: dev, test o prod. En prod no se publican /api/docs ni el spec.
	Entorno string
}

// EstadoVivo es el cuerpo de la respuesta de /healthz.
type EstadoVivo struct {
	Estado string `json:"estado" example:"ok" doc:"Siempre ok mientras el proceso responde"`

	Version string `json:"version" example:"a1b2c3d" doc:"Versión del binario"`
}

// SalidaVivo envuelve el cuerpo para Huma.
type SalidaVivo struct {
	Body EstadoVivo
}

// NuevaAPI registra las rutas en el mux y devuelve la API de Huma.
func NuevaAPI(mux *http.ServeMux, opts Opciones) huma.API {
	cfg := huma.DefaultConfig("Servicio de Stock", VersionContrato)
	cfg.CreateHooks = nil
	cfg.OpenAPIPath = "/api/openapi"
	cfg.DocsPath = "/api/docs"
	if opts.Entorno == "prod" {
		cfg.OpenAPIPath = ""
		cfg.DocsPath = ""
	}

	app := humago.New(mux, cfg)
	registrarSalud(app, opts)
	return app
}

func registrarSalud(app huma.API, opts Opciones) {
	var op huma.Operation
	op.OperationID = "obtener-vivo"
	op.Method = http.MethodGet
	op.Path = "/healthz"
	op.Summary = "Indica si el proceso está vivo"
	op.Tags = []string{"salud"}

	huma.Register(app, op, func(_ context.Context, _ *struct{}) (*SalidaVivo, error) {
		salida := &SalidaVivo{}
		salida.Body.Estado = "ok"
		salida.Body.Version = opts.Version
		return salida, nil
	})
}

// GenerarContrato devuelve el OpenAPI del servicio en YAML.
func GenerarContrato() ([]byte, error) {
	app := NuevaAPI(http.NewServeMux(), Opciones{Entorno: "dev"})
	return app.OpenAPI().YAML()
}
