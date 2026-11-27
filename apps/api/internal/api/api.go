package api

import (
	"context"
	"errors"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humago"
	"github.com/pingeso/stock/apps/api/internal/auth"
	"github.com/pingeso/stock/apps/api/internal/service"
)

type Opciones struct {
	Ambiente, Cookie, Frontend string
	Version                    string
	Validador                  *auth.Validador
	Listo                      func(context.Context) error
}

func Nuevo(s *service.Servicio, opciones Opciones) (http.Handler, huma.API) {
	mux := http.NewServeMux()
	configuracion := huma.DefaultConfig("STOCK · Grupo 8", "1.0.0")
	configuracion.DocsPath = "/api/docs"
	configuracion.OpenAPIPath = "/api/openapi"
	configuracion.SchemasPath = ""
	configuracion.Info.Description = "Entradas, salidas, ajustes con control de versión e historial de inventario. Productos es el dueño del catálogo."
	if opciones.Ambiente == "prod" {
		configuracion.DocsPath = ""
		configuracion.OpenAPIPath = ""
	}
	configuracion.Components.SecuritySchemes = map[string]*huma.SecurityScheme{
		"sesion": {Type: "apiKey", In: "cookie", Name: opciones.Cookie, Description: "JWT emitido por Identidad en cookie HttpOnly, Secure, SameSite=Lax."},
		"bearer": {Type: "http", Scheme: "bearer", BearerFormat: "JWT", Description: "Para llamadas entre servicios: reenviar JWT del usuario."},
	}
	a := humago.New(mux, configuracion)
	a.UseMiddleware(autenticar(a, opciones))
	registrarSalud(a, opciones.Version, opciones.Listo)
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		permitidos := ""
		switch r.URL.Path {
		case "/api/v1/existencias", "/healthz", "/readyz":
			permitidos = "GET, HEAD"
		case "/api/v1/movimientos":
			permitidos = "GET, HEAD, POST"
		}
		if permitidos != "" {
			w.Header().Set("Allow", permitidos)
			escribirProblema(w, http.StatusMethodNotAllowed, "Método no permitido")
			return
		}
		escribirProblema(w, http.StatusNotFound, "Ruta no encontrada")
	})
	h := &handlers{servicio: s}
	huma.Register(a, operacion("listar-existencias", http.MethodGet, "/api/v1/existencias", "Listar existencias registradas", "stock:consultar"), h.listar)
	huma.Register(a, operacion("consultar-existencia", http.MethodGet, "/api/v1/existencias/{producto_id}", "Consultar stock de un producto", "stock:consultar"), h.consultar)
	huma.Register(a, operacion("listar-movimientos", http.MethodGet, "/api/v1/movimientos", "Consultar historial de movimientos", "stock:consultar"), h.historial)
	op := operacion("crear-movimiento", http.MethodPost, "/api/v1/movimientos", "Registrar entrada, salida o ajuste", "stock:operar")
	op.DefaultStatus = http.StatusCreated
	op.Description = "Entrada/salida requieren stock:operar; ajuste requiere stock:administrar y version_esperada. Repetir la misma clave, usuario y contenido devuelve el mismo movimiento (201); otra solicitud con esa clave devuelve 409. La reversa se registra como una operación inversa con una clave nueva y la referencia al movimiento original."
	op.Extensions = map[string]any{"x-permisos": map[string]string{"entrada": "stock:operar", "salida": "stock:operar", "ajuste": "stock:administrar"}}
	huma.Register(a, op, h.mover)
	return mux, a
}

func operacion(id, metodo, ruta, resumen, permiso string) huma.Operation {
	return huma.Operation{OperationID: id, Method: metodo, Path: ruta, Summary: resumen, Tags: []string{"Stock"}, MaxBodyBytes: 16384, Errors: []int{400, 401, 403, 404, 409, 422, 500, 503}, Security: []map[string][]string{{"sesion": {}}, {"bearer": {}}}, Metadata: map[string]any{"permiso": permiso}, Extensions: map[string]any{"x-permiso": permiso}}
}

type handlers struct{ servicio *service.Servicio }

func (h *handlers) consultar(ctx context.Context, entrada *EntradaConsulta) (*SalidaExistencia, error) {
	existencia, err := h.servicio.Consultar(ctx, entrada.ProductoID)
	if err != nil {
		return nil, traducir(err)
	}
	return &SalidaExistencia{Body: convertirExistencia(existencia)}, nil
}
func (h *handlers) listar(ctx context.Context, entrada *EntradaListado) (*SalidaExistencias, error) {
	r, err := h.servicio.Listar(ctx, service.Filtro{Limite: entrada.Limite, Desplazamiento: entrada.Desplazamiento})
	if err != nil {
		return nil, traducir(err)
	}
	datos := make([]Existencia, 0, len(r.Datos))
	for _, existencia := range r.Datos {
		datos = append(datos, convertirExistencia(existencia))
	}
	return &SalidaExistencias{Body: PaginaExistencias{Datos: datos, Total: r.Total}}, nil
}
func (h *handlers) historial(ctx context.Context, entrada *EntradaHistorial) (*SalidaMovimientos, error) {
	r, err := h.servicio.Historial(ctx, service.Filtro{ProductoID: entrada.ProductoID, Tipo: entrada.Tipo, Limite: entrada.Limite, Desplazamiento: entrada.Desplazamiento})
	if err != nil {
		return nil, traducir(err)
	}
	datos := make([]Movimiento, 0, len(r.Datos))
	for _, movimiento := range r.Datos {
		datos = append(datos, convertirMovimiento(movimiento))
	}
	return &SalidaMovimientos{Body: PaginaMovimientos{Datos: datos, Total: r.Total}}, nil
}
func (h *handlers) mover(ctx context.Context, entrada *EntradaMovimiento) (*SalidaMovimiento, error) {
	usuario := auth.Usuario(ctx)
	permiso := "stock:operar"
	if entrada.Body.Tipo == "ajuste" {
		permiso = "stock:administrar"
	}
	if !usuario.Tiene(permiso) {
		return nil, huma.Error403Forbidden("No tiene permiso para este movimiento")
	}
	m, err := h.servicio.Mover(ctx, usuario.ID, entrada.Clave, service.SolicitudMovimiento{ProductoID: entrada.Body.ProductoID, Tipo: entrada.Body.Tipo, Cantidad: entrada.Body.Cantidad, VersionEsperada: entrada.Body.VersionEsperada, Motivo: entrada.Body.Motivo, Referencia: entrada.Body.Referencia})
	if err != nil {
		return nil, traducir(err)
	}
	return &SalidaMovimiento{Body: convertirMovimiento(m)}, nil
}

func convertirExistencia(e service.Existencia) Existencia {
	return Existencia{ProductoID: e.ProductoID, Cantidad: e.Cantidad, Version: e.Version, ActualizadoEn: e.ActualizadoEn}
}
func convertirMovimiento(m service.Movimiento) Movimiento {
	return Movimiento{ID: m.ID, ProductoID: m.ProductoID, Tipo: m.Tipo, Cantidad: m.Cantidad, SaldoAnterior: m.SaldoAnterior, SaldoPosterior: m.SaldoPosterior, Motivo: m.Motivo, Referencia: m.Referencia, ActorID: m.ActorID, CreadoEn: m.CreadoEn}
}

func traducir(err error) error {
	switch {
	case errors.Is(err, service.ErrNoEncontrado):
		return huma.Error404NotFound("El producto no existe")
	case errors.Is(err, service.ErrSinStock):
		return huma.Error409Conflict("Stock insuficiente")
	case errors.Is(err, service.ErrConflicto):
		return huma.Error409Conflict("La clave de idempotencia ya se utilizó con otro contenido")
	case errors.Is(err, service.ErrVersion):
		return huma.Error409Conflict("La existencia cambió; consulte la versión antes de ajustar")
	case errors.Is(err, service.ErrInvalido):
		return huma.Error422UnprocessableEntity("Movimiento inválido: verifique cantidad, motivo y versión del ajuste")
	case errors.Is(err, service.ErrProveedor):
		return huma.Error503ServiceUnavailable("Productos no está disponible; no se confirmó el movimiento")
	default:
		return huma.Error503ServiceUnavailable("No fue posible completar la operación; reintente con la misma clave")
	}
}

func registrarSalud(a huma.API, version string, listo func(context.Context) error) {
	huma.Register(a, huma.Operation{OperationID: "salud", Method: http.MethodGet, Path: "/healthz", Summary: "Comprobar proceso", Tags: []string{"Salud"}}, func(ctx context.Context, _ *struct{}) (*SalidaSalud, error) {
		r := &SalidaSalud{}
		r.Body.Estado = "ok"
		r.Body.Version = version
		return r, nil
	})
	huma.Register(a, huma.Operation{OperationID: "disponibilidad", Method: http.MethodGet, Path: "/readyz", Summary: "Comprobar PostgreSQL", Tags: []string{"Salud"}, Errors: []int{503}}, func(ctx context.Context, _ *struct{}) (*SalidaSalud, error) {
		if listo == nil || listo(ctx) != nil {
			return nil, huma.Error503ServiceUnavailable("PostgreSQL no disponible")
		}
		r := &SalidaSalud{}
		r.Body.Estado = "ok"
		r.Body.Version = version
		return r, nil
	})
}
