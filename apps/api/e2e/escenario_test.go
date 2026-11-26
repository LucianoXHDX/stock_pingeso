//go:build e2e

package e2e

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/jackc/pgx/v5"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

const (
	productoUno     = "10000000-0000-4000-8000-000000000001"
	productoDos     = "10000000-0000-4000-8000-000000000002"
	productoTres    = "10000000-0000-4000-8000-000000000003"
	productoAusente = "10000000-0000-4000-8000-000000000404"
	actorAdmin      = "20000000-0000-4000-8000-000000000001"
	origenPermitido = "http://localhost:5120"
	requestID       = "e2e-correlacion"
)

type escenario struct {
	base               string
	db, admin          *sql.DB
	nombreDB           string
	clave              ed25519.PrivateKey
	cliente            *http.Client
	tokenAdmin         string
	modoCatalogo       atomic.Int32
	consultasCatalogo  atomic.Int64
	cabecerasInvalidas atomic.Int64
	ultimaCredencial   atomic.Value
	proceso            *exec.Cmd
	entrada            io.WriteCloser
	terminado          chan error
	cerrar             sync.Once
	log                string
}

type respuesta struct {
	Estado    int
	Cuerpo    []byte
	Cabeceras http.Header
}

func iniciarEscenario(t *testing.T) *escenario {
	t.Helper()
	binario := os.Getenv("STOCK_API_BINARY")
	if binario == "" || !filepath.IsAbs(binario) {
		t.Fatal("STOCK_API_BINARY debe indicar el binario real de la API con ruta absoluta; ejecute pnpm test:e2e:api")
	}
	base := os.Getenv("STOCK_TEST_API_URL")
	if base == "" {
		base = "http://127.0.0.1:8080"
		if runtime.GOOS == "windows" {
			// Go escucha en IPv6; un servicio previo de Windows puede ocupar
			// únicamente IPv4 en el mismo puerto. El puerto interno sigue en 8080.
			base = "http://[::1]:8080"
		}
	}
	e := &escenario{base: base, cliente: &http.Client{Timeout: 12 * time.Second}, terminado: make(chan error, 1)}
	dsn, admin, nombre := prepararBase(t)
	e.admin, e.nombreDB = admin, nombre
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatal(err)
	}
	e.db = db
	t.Cleanup(func() { _ = db.Close() })
	if err := goose.SetDialect("postgres"); err != nil {
		t.Fatal(err)
	}
	if err := goose.Up(db, filepath.Join("..", "db", "migrations")); err != nil {
		t.Fatalf("migraciones reales: %v", err)
	}
	publica, privada, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	e.clave = privada
	jwks := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"keys": []any{map[string]any{"kty": "OKP", "crv": "Ed25519", "x": base64.RawURLEncoding.EncodeToString(publica), "kid": "e2e-key", "alg": "EdDSA", "use": "sig"}}})
	}))
	t.Cleanup(jwks.Close)
	catalogo := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		e.consultasCatalogo.Add(1)
		e.ultimaCredencial.Store(r.Header.Get("Authorization"))
		if !strings.HasPrefix(r.Header.Get("Authorization"), "Bearer ") || r.Header.Get("X-Request-Id") != requestID {
			e.cabecerasInvalidas.Add(1)
		}
		w.Header().Set("Content-Type", "application/json")
		modo := e.modoCatalogo.Load()
		if modo == -1 {
			select {
			case <-r.Context().Done():
				return
			case <-time.After(7 * time.Second):
			}
		}
		if modo == -2 {
			_, _ = io.WriteString(w, `{"id":"respuesta-invalida"}`)
			return
		}
		if modo > 0 {
			w.WriteHeader(int(modo))
			_, _ = io.WriteString(w, `{"detail":"proveedor no disponible"}`)
			return
		}
		id := strings.TrimPrefix(r.URL.Path, "/api/v1/productos/")
		if id == productoAusente {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if id != productoUno && id != productoDos && id != productoTres {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]string{"id": id})
	}))
	t.Cleanup(catalogo.Close)
	e.tokenAdmin = e.token(t, actorAdmin, []string{"stock:consultar", "stock:operar", "stock:administrar"}, nil)
	archivo, err := os.Create(filepath.Join(t.TempDir(), "api.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	e.log = archivo.Name()
	t.Cleanup(func() { _ = archivo.Close() })
	cmd := exec.Command(binario)
	cmd.Stdout, cmd.Stderr = archivo, archivo
	cmd.Env = append(os.Environ(),
		"API_ENV=test", "API_PORT=8080", "DATABASE_URL="+dsn,
		"AUTH_JWKS_URL="+jwks.URL, "AUTH_ISSUER=http://identidad.test", "AUTH_AUDIENCE=stock",
		"PRODUCTOS_URL="+catalogo.URL, "FRONTEND_URL="+origenPermitido, "SESSION_COOKIE_NAME=sesion", "STOCK_E2E_STDIN=1",
	)
	if os.Getenv("TZ") == "" {
		cmd.Env = append(cmd.Env, "TZ=America/Santiago")
	}
	entrada, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	e.entrada, e.proceso = entrada, cmd
	if err := cmd.Start(); err != nil {
		t.Fatalf("iniciar binario: %v", err)
	}
	go func() { e.terminado <- cmd.Wait() }()
	t.Cleanup(func() { e.detener(t) })
	limite := time.Now().Add(20 * time.Second)
	ultimoResultado := "sin solicitudes"
	for time.Now().Before(limite) {
		select {
		case err := <-e.terminado:
			e.terminado <- err
			e.mostrarLog(t)
			t.Fatalf("la API terminó antes de estar lista: %v", err)
		default:
		}
		r, err := e.solicitar(http.MethodGet, "/healthz", "", "", nil, false, "")
		if err == nil && r.Estado == http.StatusOK {
			return e
		}
		if err != nil {
			ultimoResultado = err.Error()
		} else {
			ultimoResultado = fmt.Sprintf("estado=%d cuerpo=%s", r.Estado, r.Cuerpo)
		}
		time.Sleep(50 * time.Millisecond)
	}
	e.mostrarLog(t)
	t.Fatalf("el binario no respondió /healthz en 20 segundos: %s", ultimoResultado)
	return nil
}

func prepararBase(t *testing.T) (string, *sql.DB, string) {
	t.Helper()
	ctx, cancelar := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancelar()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		var secreto [24]byte
		if _, err := rand.Read(secreto[:]); err != nil {
			t.Fatal(err)
		}
		contenedor, err := postgres.Run(ctx, "postgres:17.11",
			postgres.WithDatabase("postgres"), postgres.WithUsername("stock"), postgres.WithPassword(hex.EncodeToString(secreto[:])),
			testcontainers.WithWaitStrategy(wait.ForLog("database system is ready to accept connections").WithOccurrence(2).WithStartupTimeout(90*time.Second)),
		)
		if err != nil {
			t.Fatalf("PostgreSQL real vía Testcontainers: %v", err)
		}
		t.Cleanup(func() {
			ctx, cancelar := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancelar()
			if err := contenedor.Terminate(ctx); err != nil {
				t.Errorf("terminar PostgreSQL: %v", err)
			}
		})
		dsn, err = contenedor.ConnectionString(ctx, "sslmode=disable")
		if err != nil {
			t.Fatal(err)
		}
	}
	if _, err := pgx.ParseConfig(dsn); err != nil {
		t.Fatal("TEST_DATABASE_URL inválida")
	}
	admin, err := sql.Open("pgx", cambiarBase(dsn, "postgres"))
	if err != nil {
		t.Fatal(err)
	}
	var sufijo [8]byte
	if _, err := rand.Read(sufijo[:]); err != nil {
		t.Fatal(err)
	}
	nombre := "stock_e2e_" + hex.EncodeToString(sufijo[:])
	if _, err := admin.ExecContext(ctx, "CREATE DATABASE "+pgx.Identifier{nombre}.Sanitize()); err != nil {
		_ = admin.Close()
		t.Fatalf("crear base aislada (el usuario de TEST_DATABASE_URL requiere CREATEDB): %v", err)
	}
	t.Cleanup(func() {
		ctx, cancelar := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancelar()
		_, _ = admin.ExecContext(ctx, "ALTER DATABASE "+pgx.Identifier{nombre}.Sanitize()+" ALLOW_CONNECTIONS true")
		_, err := admin.ExecContext(ctx, "DROP DATABASE "+pgx.Identifier{nombre}.Sanitize()+" WITH (FORCE)")
		if err != nil {
			// DROP DATABASE puede completar antes de que llegue su respuesta,
			// especialmente durante un checkpoint lento de Docker Desktop.
			// Verificamos el resultado antes de declarar un fallo de limpieza.
			verificacion, terminar := context.WithTimeout(context.Background(), 10*time.Second)
			defer terminar()
			var existe bool
			consultaErr := admin.QueryRowContext(verificacion, "SELECT EXISTS (SELECT 1 FROM pg_database WHERE datname=$1)", nombre).Scan(&existe)
			if consultaErr != nil || existe {
				t.Errorf("limpiar base E2E: %v (verificación: %v; existe=%v)", err, consultaErr, existe)
			}
		}
		_ = admin.Close()
	})
	return cambiarBase(dsn, nombre), admin, nombre
}

func cambiarBase(dsn, nombre string) string {
	if strings.HasPrefix(dsn, "postgres://") || strings.HasPrefix(dsn, "postgresql://") {
		conexion, _ := url.Parse(dsn)
		conexion.Path = "/" + nombre
		conexion.RawPath = ""
		parametros := conexion.Query()
		parametros.Del("database")
		parametros.Del("dbname")
		conexion.RawQuery = parametros.Encode()
		return conexion.String()
	}
	// Los nombres son locales y se generan únicamente con letras, '_' y hex.
	return dsn + " dbname=" + nombre
}

func (e *escenario) detener(t *testing.T) {
	t.Helper()
	e.cerrar.Do(func() {
		_ = e.entrada.Close()
		select {
		case err := <-e.terminado:
			if err != nil {
				e.mostrarLog(t)
				t.Errorf("terminación de API: %v", err)
			}
		case <-time.After(8 * time.Second):
			_ = e.proceso.Process.Kill()
			<-e.terminado
			t.Error("la API no terminó limpiamente; no se garantiza la cobertura")
		}
		if t.Failed() {
			e.mostrarLog(t)
		}
	})
}

func (e *escenario) mostrarLog(t *testing.T) {
	t.Helper()
	datos, _ := os.ReadFile(e.log)
	if len(datos) > 10000 {
		datos = datos[len(datos)-10000:]
	}
	t.Logf("Log del binario (sin cuerpos ni credenciales):\n%s", datos)
}

func (e *escenario) token(t *testing.T, actor string, permisos []string, cambiar func(jwt.MapClaims)) string {
	t.Helper()
	ahora := time.Now()
	claims := jwt.MapClaims{"sub": actor, "iss": "http://identidad.test", "aud": []string{"stock"}, "iat": ahora.Add(-time.Minute).Unix(), "nbf": ahora.Add(-time.Minute).Unix(), "exp": ahora.Add(time.Hour).Unix(), "permisos": permisos}
	if cambiar != nil {
		cambiar(claims)
	}
	token := jwt.NewWithClaims(jwt.SigningMethodEdDSA, claims)
	token.Header["kid"] = "e2e-key"
	firmado, err := token.SignedString(e.clave)
	if err != nil {
		t.Fatal(err)
	}
	return firmado
}

func (e *escenario) solicitar(metodo, ruta, token, clave string, cuerpo []byte, cookie bool, origen string) (respuesta, error) {
	r, err := http.NewRequest(metodo, e.base+ruta, bytes.NewReader(cuerpo))
	if err != nil {
		return respuesta{}, err
	}
	if token != "" {
		if cookie {
			r.AddCookie(&http.Cookie{Name: "sesion", Value: token})
		} else {
			r.Header.Set("Authorization", "Bearer "+token)
		}
	}
	if clave != "" {
		r.Header.Set("Idempotency-Key", clave)
	}
	if cuerpo != nil {
		r.Header.Set("Content-Type", "application/json")
	}
	if origen != "" {
		r.Header.Set("Origin", origen)
	}
	r.Header.Set("X-Request-Id", requestID)
	res, err := e.cliente.Do(r)
	if err != nil {
		return respuesta{}, err
	}
	defer res.Body.Close()
	datos, err := io.ReadAll(io.LimitReader(res.Body, 65536))
	if err != nil {
		return respuesta{}, err
	}
	return respuesta{Estado: res.StatusCode, Cuerpo: datos, Cabeceras: res.Header}, nil
}

func (e *escenario) pedir(t *testing.T, metodo, ruta, token, clave string, cuerpo []byte, esperado int) respuesta {
	t.Helper()
	r, err := e.solicitar(metodo, ruta, token, clave, cuerpo, false, "")
	if err != nil {
		t.Fatalf("%s %s: %v", metodo, ruta, err)
	}
	comprobarEstado(t, r, esperado)
	return r
}

func comprobarEstado(t *testing.T, r respuesta, esperado int) {
	t.Helper()
	if r.Estado != esperado {
		t.Fatalf("estado recibido %d; esperado %d; cuerpo: %s", r.Estado, esperado, r.Cuerpo)
	}
	if r.Cabeceras.Get("X-Request-Id") != requestID {
		t.Fatal("no se propagó X-Request-Id")
	}
	if esperado >= 400 {
		if !strings.HasPrefix(r.Cabeceras.Get("Content-Type"), "application/problem+json") {
			t.Fatalf("error sin RFC 9457: %v", r.Cabeceras)
		}
		texto := string(r.Cuerpo)
		for _, prohibido := range []string{"SELECT ", "INSERT ", "goroutine ", "Authorization", "Bearer ", "stock_e2e_"} {
			if strings.Contains(texto, prohibido) {
				t.Fatalf("detalle interno filtrado en error: %s", prohibido)
			}
		}
	}
}

func cuerpoMovimiento(producto, tipo string, cantidad int64, version *int64) []byte {
	cuerpo := map[string]any{"producto_id": producto, "tipo": tipo, "cantidad": cantidad, "motivo": "Operación de prueba E2E", "referencia": "E2E-001"}
	if version != nil {
		cuerpo["version_esperada"] = *version
	}
	datos, err := json.Marshal(cuerpo)
	if err != nil {
		panic(fmt.Sprintf("serializar fixture: %v", err))
	}
	return datos
}

func decodificar[T any](t *testing.T, r respuesta) T {
	t.Helper()
	var valor T
	if err := json.Unmarshal(r.Cuerpo, &valor); err != nil {
		t.Fatalf("JSON de API inválido: %v: %s", err, r.Cuerpo)
	}
	return valor
}
