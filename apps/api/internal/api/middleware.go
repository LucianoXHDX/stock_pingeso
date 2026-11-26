package api

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/pingeso/stock/apps/api/internal/auth"
)

func autenticar(a huma.API, opciones Opciones) func(huma.Context, func(huma.Context)) {
	return func(ctx huma.Context, siguiente func(huma.Context)) {
		permiso, requiere := ctx.Operation().Metadata["permiso"].(string)
		if !requiere {
			siguiente(ctx)
			return
		}
		credencial := ""
		usaCookie := false
		cabecera := ctx.Header("Authorization")
		if strings.HasPrefix(cabecera, "Bearer ") {
			credencial = strings.TrimPrefix(cabecera, "Bearer ")
		} else if cookie, err := huma.ReadCookie(ctx, opciones.Cookie); err == nil && cookie != nil {
			credencial = cookie.Value
			usaCookie = true
		}
		if credencial == "" || opciones.Validador == nil {
			_ = huma.WriteErr(a, ctx, http.StatusUnauthorized, "Se requiere una sesión válida")
			return
		}
		identidad, err := opciones.Validador.Validar(ctx.Context(), credencial)
		if err != nil {
			_ = huma.WriteErr(a, ctx, http.StatusUnauthorized, "Sesión inválida o expirada")
			return
		}
		if ctx.Method() == http.MethodPost {
			origen := ctx.Header("Origin")
			if (usaCookie && origen == "") || (origen != "" && origen != opciones.Frontend) {
				_ = huma.WriteErr(a, ctx, http.StatusForbidden, "Origen no permitido")
				return
			}
			// El permiso de ajuste se conoce después de validar el cuerpo.
			if !identidad.Tiene("stock:operar") && !identidad.Tiene("stock:administrar") {
				_ = huma.WriteErr(a, ctx, http.StatusForbidden, "No tiene permiso para modificar stock")
				return
			}
		} else if !identidad.Tiene(permiso) {
			_ = huma.WriteErr(a, ctx, http.StatusForbidden, "No tiene permiso para consultar stock")
			return
		}
		siguiente(huma.WithContext(ctx, auth.ConIdentidad(ctx.Context(), identidad, credencial)))
	}
}

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

func Observar(handler http.Handler, logger *slog.Logger, version string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		inicio := time.Now()
		id := r.Header.Get("X-Request-Id")
		if !patronRequestID.MatchString(id) {
			var bytes [16]byte
			_, _ = rand.Read(bytes[:])
			id = hex.EncodeToString(bytes[:])
		}
		w.Header().Set("X-Request-Id", id)
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "same-origin")
		w.Header().Set("Cache-Control", "no-store")
		respuesta := &respuestaRegistrada{ResponseWriter: w}
		defer func() {
			if recover() != nil {
				escribirProblema(respuesta, http.StatusInternalServerError, "Error interno")
			}
			estado := respuesta.estado
			if estado == 0 {
				estado = http.StatusOK
			}
			// No se incluyen query, credenciales ni cuerpo del usuario.
			ruta := r.URL.Path
			if strings.HasPrefix(ruta, "/api/v1/existencias/") {
				ruta = "/api/v1/existencias/{producto_id}"
			}
			logger.Info("solicitud", "servicio", "stock", "version", version, "request_id", id, "metodo", r.Method, "ruta", ruta, "estado", estado, "duracion_ms", time.Since(inicio).Milliseconds())
		}()
		contexto, cancelar := context.WithTimeout(auth.ConRequestID(r.Context(), id), 12*time.Second)
		defer cancelar()
		handler.ServeHTTP(respuesta, r.WithContext(contexto))
	})
}

func escribirProblema(w http.ResponseWriter, estado int, detalle string) {
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(estado)
	_ = json.NewEncoder(w).Encode(map[string]any{"type": "about:blank", "title": http.StatusText(estado), "status": estado, "detail": detalle})
}
