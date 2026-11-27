//go:build e2e

package e2e

import (
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func TestMatrizAcceso(t *testing.T) {
	e := iniciarEscenario(t)
	casos := []struct {
		nombre                   string
		permisos                 []string
		lectura, entrada, ajuste int
	}{
		{"sin-sesion", nil, 401, 401, 401},
		{"lector", []string{"stock:consultar"}, 200, 403, 403},
		{"operador", []string{"stock:consultar", "stock:operar"}, 200, 201, 403},
		{"administrador", []string{"stock:consultar", "stock:operar", "stock:administrar"}, 200, 201, 201},
		{"solo-administrar", []string{"stock:administrar"}, 403, 403, 201},
		{"sin-permisos", []string{}, 403, 403, 403},
	}
	for indice, caso := range casos {
		t.Run(caso.nombre, func(t *testing.T) {
			token := ""
			if caso.nombre != "sin-sesion" {
				token = e.token(t, fmt.Sprintf("20000000-0000-4000-8000-%012d", indice+10), caso.permisos, nil)
			}
			e.pedir(t, http.MethodGet, "/api/v1/existencias", token, "", nil, caso.lectura)
			e.pedir(t, http.MethodPost, "/api/v1/movimientos", token, "matriz-entrada-"+caso.nombre, cuerpoMovimiento(productoUno, "entrada", 1, nil), caso.entrada)
			actual := consultarStock(t, e, productoUno)
			e.pedir(t, http.MethodPost, "/api/v1/movimientos", token, "matriz-ajuste-"+caso.nombre, cuerpoMovimiento(productoUno, "ajuste", int64(50+indice), &actual.Version), caso.ajuste)
		})
	}
	lectura := []string{"stock:consultar"}
	credencialesInvalidas := []struct {
		nombre  string
		cambiar func(jwt.MapClaims)
	}{
		{"expirado", func(c jwt.MapClaims) { c["exp"] = time.Now().Add(-time.Hour).Unix() }},
		{"otro-emisor", func(c jwt.MapClaims) { c["iss"] = "http://otra-identidad.test" }},
		{"otra-audiencia", func(c jwt.MapClaims) { c["aud"] = []string{"otro-servicio"} }},
		{"sin-expiracion", func(c jwt.MapClaims) { delete(c, "exp") }},
		{"todavia-no-valido", func(c jwt.MapClaims) { c["nbf"] = time.Now().Add(time.Hour).Unix() }},
		{"sujeto-invalido", func(c jwt.MapClaims) { c["sub"] = "usuario-sin-uuid" }},
	}
	for _, caso := range credencialesInvalidas {
		t.Run(caso.nombre, func(t *testing.T) {
			token := e.token(t, actorAdmin, lectura, caso.cambiar)
			e.pedir(t, "GET", "/api/v1/existencias", token, "", nil, 401)
		})
	}
	t.Run("firma-alterada", func(t *testing.T) {
		partes := strings.Split(e.tokenAdmin, ".")
		if partes[2][0] == 'A' {
			partes[2] = "B" + partes[2][1:]
		} else {
			partes[2] = "A" + partes[2][1:]
		}
		e.pedir(t, "GET", "/api/v1/existencias", strings.Join(partes, "."), "", nil, 401)
	})
	t.Run("algoritmo-no-permitido", func(t *testing.T) {
		token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{"sub": actorAdmin, "iss": "http://identidad.test", "aud": []string{"stock"}, "exp": time.Now().Add(time.Hour).Unix(), "permisos": lectura})
		token.Header["kid"] = "e2e-key"
		firmado, err := token.SignedString([]byte("fixture-temporal-para-algoritmo-rechazado"))
		if err != nil {
			t.Fatal(err)
		}
		e.pedir(t, "GET", "/api/v1/existencias", firmado, "", nil, 401)
	})
	origenes := []struct {
		nombre string
		cookie bool
		origen string
		estado int
	}{
		{"cookie-origen-correcto", true, origenPermitido, 201},
		{"cookie-sin-origen", true, "", 403},
		{"cookie-origen-ajeno", true, "https://otro-origen.test", 403},
		{"bearer-sin-origen", false, "", 201},
		{"bearer-origen-ajeno", false, "https://otro-origen.test", 403},
	}
	for _, caso := range origenes {
		t.Run(caso.nombre, func(t *testing.T) {
			r, err := e.solicitar("POST", "/api/v1/movimientos", e.tokenAdmin, "origen-"+caso.nombre, cuerpoMovimiento(productoUno, "entrada", 1, nil), caso.cookie, caso.origen)
			if err != nil {
				t.Fatal(err)
			}
			comprobarEstado(t, r, caso.estado)
		})
	}
	t.Run("cookie-lectura", func(t *testing.T) {
		r, err := e.solicitar("GET", "/api/v1/existencias", e.tokenAdmin, "", nil, true, "")
		if err != nil {
			t.Fatal(err)
		}
		comprobarEstado(t, r, 200)
	})
	t.Run("clave-obligatoria", func(t *testing.T) {
		e.pedir(t, "POST", "/api/v1/movimientos", e.tokenAdmin, "", cuerpoMovimiento(productoUno, "entrada", 1, nil), 422)
	})
	t.Run("idempotencia-por-usuario", func(t *testing.T) {
		ids := map[string]bool{}
		for indice := 1; indice <= 2; indice++ {
			actor := fmt.Sprintf("30000000-0000-4000-8000-%012d", indice)
			token := e.token(t, actor, []string{"stock:operar"}, nil)
			r := e.pedir(t, "POST", "/api/v1/movimientos", token, "clave-compartida-por-usuario", cuerpoMovimiento(productoUno, "entrada", 1, nil), 201)
			movimiento := decodificar[movimientoAPI](t, r)
			ids[movimiento.ID] = true
			if movimiento.ActorID != actor {
				t.Fatal("el movimiento se atribuyó a otro usuario")
			}
		}
		if len(ids) != 2 {
			t.Fatal("dos usuarios con la misma clave deben poder registrar operaciones distintas")
		}
	})
	if e.cabecerasInvalidas.Load() != 0 {
		t.Fatal("las credenciales de cookie no se reenviaron como Bearer a Productos")
	}
}
