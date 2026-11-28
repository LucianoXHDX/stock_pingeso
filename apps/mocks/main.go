// Command mocks simula Identidad y Productos para desarrollo local de STOCK.
//
// Sólo para el perfil de desarrollo: genera una llave Ed25519 nueva en cada arranque,
// publica su JWKS, emite sesiones de prueba por perfil y responde el contrato propuesto
// de Productos (contracts/externos/productos.yaml). No se publica ni se usa en CI.
package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"html/template"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"
)

const duracionSesion = 8 * time.Hour

// Perfiles alineados con docs/integracion.md. Los permisos son explícitos.
var perfiles = map[string][]string{
	"admin":     {"stock:consultar", "stock:operar", "stock:administrar"},
	"operacion": {"stock:consultar", "stock:operar"},
	"consulta":  {"stock:consultar"},
}

// Usuarios ficticios, uno por perfil, para que el historial muestre actores distintos.
var usuarios = map[string]string{
	"admin":     "20000000-0000-4000-8000-000000000001",
	"operacion": "20000000-0000-4000-8000-000000000002",
	"consulta":  "20000000-0000-4000-8000-000000000003",
}

type producto struct {
	ID     string `json:"id"`
	Nombre string `json:"nombre"`
}

// El primero coincide con apps/api/db/seed.sql.
var productosPorDefecto = []producto{
	{"10000000-0000-4000-8000-000000000001", "Producto demo (seed)"},
	{"10000000-0000-4000-8000-000000000002", "Cuaderno universitario"},
	{"10000000-0000-4000-8000-000000000003", "Lápiz grafito"},
	{"10000000-0000-4000-8000-000000000004", "Resma carta"},
	{"10000000-0000-4000-8000-000000000005", "Corchetera"},
}

var patronUUID = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

type configuracion struct {
	emisor, audiencia, cookie, frontend string
	productos                           []producto
}

type mock struct {
	cfg     configuracion
	privada ed25519.PrivateKey
	publica ed25519.PublicKey
	kid     string
	log     *slog.Logger
}

func main() {
	salud := flag.Bool("healthcheck", false, "comprueba la salud del mock")
	flag.Parse()
	if *salud {
		os.Exit(comprobarSalud())
	}

	log := slog.New(slog.NewJSONHandler(os.Stdout, nil)).With("servicio", "mocks")
	cfg, err := cargar()
	if err != nil {
		log.Error("configuración inválida", "causa", err.Error())
		os.Exit(1)
	}
	m, err := nuevo(cfg, log)
	if err != nil {
		log.Error("no se pudo generar la llave", "causa", err.Error())
		os.Exit(1)
	}
	servidor := &http.Server{Addr: ":8080", Handler: m.rutas(), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 10 * time.Second, IdleTimeout: 60 * time.Second}
	log.Info("mocks de Identidad y Productos iniciados", "puerto", 8080, "emisor", cfg.emisor, "audiencia", cfg.audiencia, "kid", m.kid)
	if err := servidor.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Error("el mock terminó con error", "causa", err.Error())
		os.Exit(1)
	}
}

func comprobarSalud() int {
	cliente := http.Client{Timeout: 2 * time.Second}
	respuesta, err := cliente.Get("http://127.0.0.1:8080/healthz")
	if err != nil {
		return 1
	}
	defer respuesta.Body.Close()
	if respuesta.StatusCode != http.StatusOK {
		return 1
	}
	return 0
}

func variable(nombre, porDefecto string) string {
	if valor := strings.TrimSpace(os.Getenv(nombre)); valor != "" {
		return valor
	}
	return porDefecto
}

func cargar() (configuracion, error) {
	cfg := configuracion{
		emisor:    variable("MOCK_EMISOR", "http://identidad-mock"),
		audiencia: variable("MOCK_AUDIENCIA", "stock"),
		cookie:    variable("MOCK_COOKIE", "sesion"),
		frontend:  strings.TrimRight(variable("FRONTEND_URL", "http://localhost:5120"), "/"),
		productos: productosPorDefecto,
	}
	if u, err := url.Parse(cfg.frontend); err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
		return cfg, fmt.Errorf("FRONTEND_URL debe ser una URL HTTP válida")
	}
	if err := (&http.Cookie{Name: cfg.cookie, Value: "x"}).Valid(); err != nil {
		return cfg, fmt.Errorf("MOCK_COOKIE inválido")
	}
	// MOCK_PRODUCTOS permite reemplazar el catálogo: "uuid=Nombre,uuid=Nombre".
	if lista := os.Getenv("MOCK_PRODUCTOS"); strings.TrimSpace(lista) != "" {
		cfg.productos = nil
		for _, par := range strings.Split(lista, ",") {
			id, nombre, _ := strings.Cut(strings.TrimSpace(par), "=")
			if !patronUUID.MatchString(id) {
				return cfg, fmt.Errorf("MOCK_PRODUCTOS contiene un UUID inválido: %q", id)
			}
			if nombre == "" {
				nombre = id
			}
			cfg.productos = append(cfg.productos, producto{ID: strings.ToLower(id), Nombre: nombre})
		}
	}
	return cfg, nil
}

func nuevo(cfg configuracion, log *slog.Logger) (*mock, error) {
	publica, privada, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, err
	}
	huella := sha256.Sum256(publica)
	return &mock{cfg: cfg, privada: privada, publica: publica, kid: "mock-" + base64.RawURLEncoding.EncodeToString(huella[:8]), log: log}, nil
}

func (m *mock) rutas() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		escribirJSON(w, http.StatusOK, map[string]string{"estado": "ok"})
	})
	mux.HandleFunc("GET /{$}", m.inicio)
	// Identidad
	mux.HandleFunc("GET /.well-known/jwks.json", m.jwks)
	mux.HandleFunc("GET /dev/sesion", m.sesion)
	mux.HandleFunc("GET /dev/salir", m.salir)
	mux.HandleFunc("GET /dev/token", m.tokenTexto)
	// Productos: contrato propuesto en contracts/externos/productos.yaml
	mux.HandleFunc("GET /api/v1/productos/{producto_id}", m.obtenerProducto)
	mux.HandleFunc("GET /api/v1/productos", m.listarProductos)
	return m.observar(mux)
}

func (m *mock) observar(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		inicio := time.Now()
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Cache-Control", "no-store")
		h.ServeHTTP(w, r)
		// No se registran tokens, cookies ni consultas.
		m.log.Info("solicitud", "metodo", r.Method, "ruta", r.URL.Path, "request_id", r.Header.Get("X-Request-Id"), "duracion_ms", time.Since(inicio).Milliseconds())
	})
}

func (m *mock) jwks(w http.ResponseWriter, _ *http.Request) {
	escribirJSON(w, http.StatusOK, map[string]any{"keys": []map[string]string{{
		"kty": "OKP", "crv": "Ed25519", "alg": "EdDSA", "use": "sig", "kid": m.kid,
		"x": base64.RawURLEncoding.EncodeToString(m.publica),
	}}})
}

func (m *mock) firmar(perfil string) (string, error) {
	permisos, ok := perfiles[perfil]
	if !ok {
		return "", fmt.Errorf("perfil desconocido")
	}
	ahora := time.Now().UTC()
	cabecera := map[string]string{"alg": "EdDSA", "typ": "JWT", "kid": m.kid}
	claims := map[string]any{
		"sub": usuarios[perfil], "iss": m.cfg.emisor, "aud": m.cfg.audiencia,
		"iat": ahora.Unix(), "nbf": ahora.Unix(), "exp": ahora.Add(duracionSesion).Unix(),
		"permisos": permisos, "perfil": perfil,
	}
	c, err := json.Marshal(cabecera)
	if err != nil {
		return "", err
	}
	p, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}
	firmable := base64.RawURLEncoding.EncodeToString(c) + "." + base64.RawURLEncoding.EncodeToString(p)
	firma := ed25519.Sign(m.privada, []byte(firmable))
	return firmable + "." + base64.RawURLEncoding.EncodeToString(firma), nil
}

func perfilSolicitado(r *http.Request) string {
	return variableConsulta(r, "perfil", "admin")
}

func variableConsulta(r *http.Request, nombre, porDefecto string) string {
	if v := r.URL.Query().Get(nombre); v != "" {
		return v
	}
	return porDefecto
}

func (m *mock) sesion(w http.ResponseWriter, r *http.Request) {
	perfil := perfilSolicitado(r)
	token, err := m.firmar(perfil)
	if err != nil {
		problema(w, http.StatusBadRequest, "Perfil desconocido: use admin, operacion o consulta")
		return
	}
	// Mismos atributos que exige DAS para la cookie de Identidad. Las cookies no distinguen
	// puerto, por lo que la emitida en localhost:5129 llega a localhost:5120.
	http.SetCookie(w, &http.Cookie{Name: m.cfg.cookie, Value: token, Path: "/", MaxAge: int(duracionSesion.Seconds()), HttpOnly: true, Secure: true, SameSite: http.SameSiteLaxMode})
	http.Redirect(w, r, m.cfg.frontend+"/", http.StatusSeeOther)
}

func (m *mock) salir(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{Name: m.cfg.cookie, Value: "", Path: "/", MaxAge: -1, HttpOnly: true, Secure: true, SameSite: http.SameSiteLaxMode})
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (m *mock) tokenTexto(w http.ResponseWriter, r *http.Request) {
	token, err := m.firmar(perfilSolicitado(r))
	if err != nil {
		problema(w, http.StatusBadRequest, "Perfil desconocido: use admin, operacion o consulta")
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = fmt.Fprintln(w, token)
}

func (m *mock) buscar(id string) (producto, bool) {
	for _, p := range m.cfg.productos {
		if strings.EqualFold(p.ID, id) {
			return p, true
		}
	}
	return producto{}, false
}

func (m *mock) obtenerProducto(w http.ResponseWriter, r *http.Request) {
	if !strings.HasPrefix(r.Header.Get("Authorization"), "Bearer ") {
		problema(w, http.StatusUnauthorized, "Falta el JWT del usuario")
		return
	}
	id := r.PathValue("producto_id")
	if !patronUUID.MatchString(id) {
		problema(w, http.StatusNotFound, "Producto inexistente")
		return
	}
	p, ok := m.buscar(id)
	if !ok {
		problema(w, http.StatusNotFound, "Producto inexistente")
		return
	}
	escribirJSON(w, http.StatusOK, map[string]string{"id": strings.ToLower(id), "nombre": p.Nombre})
}

// listarProductos no es parte del contrato: ayuda a saber qué UUID usar en las pruebas.
func (m *mock) listarProductos(w http.ResponseWriter, _ *http.Request) {
	escribirJSON(w, http.StatusOK, map[string]any{"datos": m.cfg.productos, "nota": "Mock local de Productos; no es el contrato oficial del grupo 3"})
}

var plantilla = template.Must(template.New("inicio").Parse(`<!doctype html>
<html lang="es"><head><meta charset="utf-8"><title>Mocks STOCK</title>
<style>body{font-family:system-ui,sans-serif;max-width:46rem;margin:2rem auto;padding:0 1rem;line-height:1.5}
code{background:#f2f2f2;padding:.1rem .3rem;border-radius:4px}a.b{display:inline-block;margin:.25rem .5rem .25rem 0;padding:.5rem .9rem;border:1px solid #333;border-radius:6px;text-decoration:none;color:#111}
table{border-collapse:collapse}td,th{border:1px solid #ccc;padding:.3rem .6rem;text-align:left}</style></head>
<body><h1>Mocks locales de STOCK</h1>
<p>Sólo para desarrollo. Simula <strong>Identidad</strong> (grupo 6) y <strong>Productos</strong> (grupo 3).</p>
<h2>1. Iniciar sesión de prueba</h2>
<p>Crea la cookie <code>{{.Cookie}}</code> y vuelve a <code>{{.Frontend}}</code>.</p>
<p><a class="b" href="/dev/sesion?perfil=admin">Admin</a><a class="b" href="/dev/sesion?perfil=operacion">Operación</a><a class="b" href="/dev/sesion?perfil=consulta">Consulta</a><a class="b" href="/dev/salir">Cerrar sesión</a></p>
<table><tr><th>Perfil</th><th>Permisos</th></tr>{{range $p, $permisos := .Perfiles}}<tr><td>{{$p}}</td><td>{{range $permisos}}<code>{{.}}</code> {{end}}</td></tr>{{end}}</table>
<h2>2. Productos que existen en el mock</h2>
<table><tr><th>UUID</th><th>Nombre</th></tr>{{range .Productos}}<tr><td><code>{{.ID}}</code></td><td>{{.Nombre}}</td></tr>{{end}}</table>
<p>Cualquier otro UUID responde 404 (producto inexistente).</p>
<h2>3. Token para curl o Postman</h2>
<p><a href="/dev/token?perfil=admin"><code>/dev/token?perfil=admin</code></a> devuelve un JWT para usar como <code>Authorization: Bearer</code>. Las sesiones duran 8 horas y se invalidan al reiniciar este contenedor.</p>
</body></html>`))

func (m *mock) inicio(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = plantilla.Execute(w, map[string]any{"Cookie": m.cfg.cookie, "Frontend": m.cfg.frontend, "Perfiles": perfiles, "Productos": m.cfg.productos})
}

func escribirJSON(w http.ResponseWriter, estado int, cuerpo any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(estado)
	_ = json.NewEncoder(w).Encode(cuerpo)
}

func problema(w http.ResponseWriter, estado int, detalle string) {
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(estado)
	_ = json.NewEncoder(w).Encode(map[string]any{"type": "about:blank", "title": http.StatusText(estado), "status": estado, "detail": detalle})
}
