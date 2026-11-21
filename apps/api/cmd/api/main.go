package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humago"
)

// version se inyecta al compilar con -ldflags "-X main.version=...".
var version = "dev"

const puertoPorDefecto = "8080"

const tiempoCierre = 10 * time.Second

// EstadoVivo es el cuerpo de la respuesta de /healthz.
type EstadoVivo struct {
	Estado string `json:"estado" example:"ok" doc:"Siempre ok mientras el proceso responde"`

	Version string `json:"version" example:"a1b2c3d" doc:"Versión del binario"`
}

// SalidaVivo envuelve el cuerpo para Huma.
type SalidaVivo struct {
	Body EstadoVivo
}

// puerto lee API_PORT. Pendiente: reemplazar por el paquete config, que valida al arrancar.
func puerto() string {
	if valor := os.Getenv("API_PORT"); valor != "" {
		return valor
	}
	return puertoPorDefecto
}

func nuevaAPI(mux *http.ServeMux) huma.API {
	cfg := huma.DefaultConfig("API de Stock", "1.0.0")
	cfg.CreateHooks = nil
	api := humago.New(mux, cfg)

	var op huma.Operation
	op.OperationID = "obtener-vivo"
	op.Method = http.MethodGet
	op.Path = "/healthz"
	op.Summary = "Indica si el proceso está vivo"
	op.Tags = []string{"salud"}

	huma.Register(api, op, func(_ context.Context, _ *struct{}) (*SalidaVivo, error) {
		salida := &SalidaVivo{}
		salida.Body.Estado = "ok"
		salida.Body.Version = version
		return salida, nil
	})

	return api
}

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil)).With("servicio", "stock", "version", version)
	slog.SetDefault(log)

	if err := ejecutar(log); err != nil {
		log.Error("el servicio terminó con error", "error", err)
		os.Exit(1)
	}
}

func ejecutar(log *slog.Logger) error {
	mux := http.NewServeMux()
	nuevaAPI(mux)

	servidor := &http.Server{}
	servidor.Addr = ":" + puerto()
	servidor.Handler = mux
	servidor.ReadHeaderTimeout = 5 * time.Second

	ctx, parar := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer parar()

	errores := make(chan error, 1)
	go func() {
		errores <- servidor.ListenAndServe()
	}()
	log.Info("servicio iniciado", "puerto", puerto())

	select {
	case err := <-errores:
		if !errors.Is(err, http.ErrServerClosed) {
			return err
		}
	case <-ctx.Done():
		log.Info("cerrando el servicio")
	}

	ctxCierre, cancelar := context.WithTimeout(context.Background(), tiempoCierre)
	defer cancelar()
	return servidor.Shutdown(ctxCierre)
}
