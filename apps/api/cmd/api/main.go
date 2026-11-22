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

	"stock/internal/api"
)

// version se inyecta al compilar con -ldflags "-X main.version=...".
var version = "dev"

const puertoPorDefecto = "8080"

const entornoPorDefecto = "dev"

const tiempoCierre = 10 * time.Second

// variable lee una variable de entorno. Pendiente: reemplazar por el paquete config.
func variable(nombre, porDefecto string) string {
	if valor := os.Getenv(nombre); valor != "" {
		return valor
	}
	return porDefecto
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
	puerto := variable("API_PORT", puertoPorDefecto)

	mux := http.NewServeMux()
	api.NuevaAPI(mux, api.Opciones{Version: version, Entorno: variable("API_ENV", entornoPorDefecto)})

	servidor := &http.Server{}
	servidor.Addr = ":" + puerto
	servidor.Handler = mux
	servidor.ReadHeaderTimeout = 5 * time.Second

	ctx, parar := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer parar()

	errores := make(chan error, 1)
	go func() {
		errores <- servidor.ListenAndServe()
	}()
	log.Info("servicio iniciado", "puerto", puerto)

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
