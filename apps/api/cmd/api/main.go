package main

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
	_ "time/tzdata"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pingeso/stock/apps/api/internal/api"
	"github.com/pingeso/stock/apps/api/internal/auth"
	"github.com/pingeso/stock/apps/api/internal/config"
	"github.com/pingeso/stock/apps/api/internal/productos"
	"github.com/pingeso/stock/apps/api/internal/service"
	"github.com/pingeso/stock/apps/api/internal/store"
)

var version = "dev"

func main() {
	if len(os.Args) == 2 && os.Args[1] == "-healthcheck" {
		cliente := http.Client{Timeout: 2 * time.Second}
		respuesta, err := cliente.Get("http://127.0.0.1:8080/healthz")
		if err != nil {
			os.Exit(1)
		}
		defer respuesta.Body.Close()
		if respuesta.StatusCode != http.StatusOK {
			os.Exit(1)
		}
		return
	}
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	if err := ejecutar(logger); err != nil {
		logger.Error("No se pudo iniciar STOCK", "causa", err.Error())
		os.Exit(1)
	}
}

func ejecutar(logger *slog.Logger) error {
	c, err := config.Cargar()
	if err != nil {
		return err
	}
	ctx, cancelar := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancelar()
	// Permite que el proceso E2E cierre limpiamente el binario también en Windows
	// y que Go escriba la cobertura. No se habilita en dev ni prod.
	if c.Ambiente == "test" && os.Getenv("STOCK_E2E_STDIN") == "1" {
		go func() { _, _ = io.Copy(io.Discard, os.Stdin); cancelar() }()
	}
	pool, err := pgxpool.New(ctx, c.BaseDatos)
	if err != nil {
		return configError("DATABASE_URL inválida")
	}
	defer pool.Close()
	ctxInicio, cancelarInicio := context.WithTimeout(ctx, 5*time.Second)
	err = pool.Ping(ctxInicio)
	cancelarInicio()
	if err != nil {
		return configError("PostgreSQL no disponible")
	}
	validador, err := auth.Nuevo(ctx, c.JWKS, c.Emisor, c.Audiencia)
	if err != nil {
		return err
	}
	servicio := service.Nuevo(store.Nuevo(pool), productos.Nuevo(c.Productos))
	handler, _ := api.Nuevo(servicio, api.Opciones{Ambiente: c.Ambiente, Cookie: c.Cookie, Frontend: c.Frontend, Version: version, Validador: validador, Listo: func(ctx context.Context) error {
		contexto, cancelar := context.WithTimeout(ctx, 2*time.Second)
		defer cancelar()
		return pool.Ping(contexto)
	}})
	servidor := &http.Server{Addr: ":" + c.Puerto, Handler: api.Observar(handler, logger, version), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 15 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 16384}
	terminado := make(chan error, 1)
	go func() { terminado <- servidor.ListenAndServe() }()
	logger.Info("STOCK iniciado", "servicio", "stock", "version", version, "puerto", c.Puerto)
	select {
	case err := <-terminado:
		if err == http.ErrServerClosed {
			return nil
		}
		return err
	case <-ctx.Done():
		contexto, cancelar := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancelar()
		return servidor.Shutdown(contexto)
	}
}

type configError string

func (e configError) Error() string { return string(e) }
