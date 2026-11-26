package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"log/slog"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/signal"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
	"time"
)

var version = "dev"

var patronRequestID = regexp.MustCompile(`^[A-Za-z0-9._:-]{1,128}$`)

type respuestaRegistrada struct {
	http.ResponseWriter
	estado int
}

func (r *respuestaRegistrada) WriteHeader(estado int) {
	if r.estado == 0 {
		r.estado = estado
		r.ResponseWriter.WriteHeader(estado)
	}
}

func (r *respuestaRegistrada) Write(datos []byte) (int, error) {
	if r.estado == 0 {
		r.WriteHeader(http.StatusOK)
	}
	return r.ResponseWriter.Write(datos)
}

func (r *respuestaRegistrada) Unwrap() http.ResponseWriter { return r.ResponseWriter }

func observar(handler http.Handler, logger *slog.Logger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		inicio := time.Now()
		id := r.Header.Get("X-Request-Id")
		if !patronRequestID.MatchString(id) {
			var bytes [16]byte
			_, _ = rand.Read(bytes[:])
			id = hex.EncodeToString(bytes[:])
		}
		r.Header.Set("X-Request-Id", id)
		w.Header().Set("X-Request-Id", id)
		respuesta := &respuestaRegistrada{ResponseWriter: w}
		handler.ServeHTTP(respuesta, r)
		estado := respuesta.estado
		if estado == 0 {
			estado = http.StatusOK
		}
		logger.Info("solicitud", "request_id", id, "metodo", r.Method, "ruta", r.URL.Path, "estado", estado, "duracion_ms", time.Since(inicio).Milliseconds())
	})
}

func main() {
	salud := flag.Bool("healthcheck", false, "comprueba la salud del servidor")
	flag.Parse()
	if *salud {
		cliente := &http.Client{Timeout: 3 * time.Second}
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
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil)).With("servicio", "stock-web", "version", version)
	backend, err := url.Parse(os.Getenv("BACKEND_URL"))
	if err != nil || backend == nil || backend.Host == "" || (backend.Scheme != "http" && backend.Scheme != "https") || backend.User != nil || backend.RawQuery != "" || backend.Fragment != "" {
		logger.Error("BACKEND_URL debe ser una URL HTTP válida sin credenciales, consulta ni fragmento")
		os.Exit(1)
	}
	raiz := os.Getenv("SPA_DIRECTORY")
	if raiz == "" {
		raiz = "/srv"
	}
	if _, err := os.Stat(filepath.Join(raiz, "index.html")); err != nil {
		logger.Error("no se encuentra la aplicación web compilada")
		os.Exit(1)
	}
	proxy := httputil.NewSingleHostReverseProxy(backend)
	proxy.Transport = &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		DialContext:           (&net.Dialer{Timeout: 5 * time.Second, KeepAlive: 30 * time.Second}).DialContext,
		TLSHandshakeTimeout:   5 * time.Second,
		ResponseHeaderTimeout: 5 * time.Second,
		IdleConnTimeout:       90 * time.Second,
		MaxIdleConns:          100,
	}
	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		w.Header().Set("Content-Type", "application/problem+json")
		w.WriteHeader(http.StatusBadGateway)
		_ = json.NewEncoder(w).Encode(map[string]any{"type": "about:blank", "title": "Servicio de stock no disponible", "status": http.StatusBadGateway})
		logger.Warn("falló la conexión con la API", "request_id", r.Header.Get("X-Request-Id"))
	}
	manejador := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		if r.URL.Path == "/healthz" {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte("{\"estado\":\"ok\"}\n"))
			return
		}
		if r.URL.Path == "/api" || strings.HasPrefix(r.URL.Path, "/api/") {
			w.Header().Set("Cache-Control", "no-store")
			proxy.ServeHTTP(w, r)
			return
		}
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			http.Error(w, "Método no permitido", http.StatusMethodNotAllowed)
			return
		}
		ruta := path.Clean("/" + r.URL.Path)
		archivo := filepath.Join(raiz, filepath.FromSlash(strings.TrimPrefix(ruta, "/")))
		info, err := os.Stat(archivo)
		if strings.HasPrefix(ruta, "/assets/") {
			if err != nil || info.IsDir() {
				http.NotFound(w, r)
				return
			}
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		} else {
			w.Header().Set("Cache-Control", "no-cache")
			if err != nil || info.IsDir() {
				archivo = filepath.Join(raiz, "index.html")
			}
		}
		http.ServeFile(w, r, archivo)
	})
	servidor := &http.Server{Addr: ":8080", Handler: observar(manejador, logger), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 30 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 1 << 20}
	ctx, detener := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer detener()
	go func() {
		<-ctx.Done()
		cierre, cancelar := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancelar()
		if err := servidor.Shutdown(cierre); err != nil {
			logger.Error("no se pudo cerrar el servidor a tiempo")
		}
	}()
	logger.Info("servidor web iniciado", "puerto", 8080)
	if err := servidor.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		logger.Error("falló el servidor web")
		os.Exit(1)
	}
}
