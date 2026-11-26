//go:build e2e

package e2e

import (
	"bytes"
	"context"
	"crypto/rand"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

type existenciaAPI struct {
	ProductoID    string    `json:"producto_id"`
	Cantidad      int64     `json:"cantidad"`
	Version       int64     `json:"version"`
	ActualizadoEn time.Time `json:"actualizado_en"`
}

type movimientoAPI struct {
	ID             string    `json:"id"`
	ProductoID     string    `json:"producto_id"`
	Tipo           string    `json:"tipo"`
	Cantidad       int64     `json:"cantidad"`
	SaldoAnterior  int64     `json:"saldo_anterior"`
	SaldoPosterior int64     `json:"saldo_posterior"`
	ActorID        string    `json:"actor_id"`
	CreadoEn       time.Time `json:"creado_en"`
}

type paginaAPI[T any] struct {
	Datos []T   `json:"datos"`
	Total int64 `json:"total"`
}

func TestFlujoStock(t *testing.T) {
	e := iniciarEscenario(t)
	var reporte strings.Builder
	reporte.WriteString("# Resultado E2E STOCK\n\n")
	e.pedir(t, http.MethodGet, "/healthz", "", "", nil, 200)
	e.pedir(t, http.MethodGet, "/readyz", "", "", nil, 200)
	reporte.WriteString("- Salud: healthz=200; readyz=200.\n")
	inicial := decodificar[paginaAPI[existenciaAPI]](t, e.pedir(t, "GET", "/api/v1/existencias", e.tokenAdmin, "", nil, 200))
	historialInicial := decodificar[paginaAPI[movimientoAPI]](t, e.pedir(t, "GET", "/api/v1/movimientos", e.tokenAdmin, "", nil, 200))
	if inicial.Total != 0 || inicial.Datos == nil || len(inicial.Datos) != 0 || historialInicial.Total != 0 || historialInicial.Datos == nil {
		t.Fatal("la base inicial debe tener páginas vacías con arrays, sin datos sembrados")
	}
	sinMovimientos := consultarStock(t, e, productoUno)
	if sinMovimientos.Cantidad != 0 || sinMovimientos.Version != 0 {
		t.Fatal("un producto válido sin movimientos comienza en cero")
	}
	reporte.WriteString("- Estado inicial: 0 existencias y 0 movimientos; producto conocido con saldo=0.\n")
	primero := decodificar[movimientoAPI](t, e.pedir(t, "POST", "/api/v1/movimientos", e.tokenAdmin, "entrada-inicial-001", cuerpoMovimiento(productoUno, "entrada", 1, nil), 201))
	if primero.SaldoAnterior != 0 || primero.SaldoPosterior != 1 || primero.Cantidad != 1 || primero.ActorID != actorAdmin || primero.ID == "" || primero.CreadoEn.IsZero() {
		t.Fatalf("entrada inconsistente: %+v", primero)
	}
	if credencial, _ := e.ultimaCredencial.Load().(string); credencial != "Bearer "+e.tokenAdmin {
		t.Fatal("Productos no recibió el JWT del usuario")
	}

	// La demora en PostgreSQL amplía la ventana de carrera y hace que la prueba
	// detecte también actualizaciones perdidas al retirar FOR UPDATE.
	if _, err := e.db.Exec(`CREATE FUNCTION demorar_actualizacion() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN PERFORM pg_sleep(0.25); RETURN NEW; END $$;
CREATE TRIGGER demora_e2e BEFORE UPDATE ON existencias FOR EACH ROW EXECUTE FUNCTION demorar_actualizacion()`); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = e.db.Exec("DROP TRIGGER IF EXISTS demora_e2e ON existencias; DROP FUNCTION IF EXISTS demorar_actualizacion()")
	})
	resultados := enParalelo(t, 12, func(indice int) (respuesta, error) {
		return e.solicitar("POST", "/api/v1/movimientos", e.tokenAdmin, fmt.Sprintf("salida-concurrente-%02d", indice), cuerpoMovimiento(productoUno, "salida", 1, nil), false, "")
	})
	aceptadas, conflictos := 0, 0
	for _, r := range resultados {
		switch r.Estado {
		case 201:
			aceptadas++
		case 409:
			conflictos++
		default:
			t.Fatalf("salida concurrente inesperada: %d %s", r.Estado, r.Cuerpo)
		}
		comprobarEstado(t, r, r.Estado)
	}
	if _, err := e.db.Exec("DROP TRIGGER demora_e2e ON existencias; DROP FUNCTION demorar_actualizacion()"); err != nil {
		t.Fatal(err)
	}
	stock := consultarStock(t, e, productoUno)
	if aceptadas != 1 || conflictos != 11 || stock.Cantidad != 0 || stock.Version != 2 {
		t.Fatalf("carrera sobre última unidad: aceptadas=%d conflictos=%d stock=%+v", aceptadas, conflictos, stock)
	}
	fmt.Fprintf(&reporte, "- Última unidad: 12 salidas simultáneas; aceptadas=%d, conflictos=%d, saldo=%d, versión=%d.\n", aceptadas, conflictos, stock.Cantidad, stock.Version)

	cuerpoIdempotente := cuerpoMovimiento(productoDos, "entrada", 7, nil)
	claveIdempotente := "entrada-" + rand.Text()
	resultados = enParalelo(t, 8, func(_ int) (respuesta, error) {
		return e.solicitar("POST", "/api/v1/movimientos", e.tokenAdmin, claveIdempotente, cuerpoIdempotente, false, "")
	})
	ids := map[string]bool{}
	var original respuesta
	for _, r := range resultados {
		comprobarEstado(t, r, 201)
		m := decodificar[movimientoAPI](t, r)
		ids[m.ID] = true
		if original.Cuerpo == nil {
			original = r
		} else if !bytes.Equal(original.Cuerpo, r.Cuerpo) {
			t.Fatal("un replay concurrente debe devolver exactamente el mismo movimiento")
		}
	}
	stockDos := consultarStock(t, e, productoDos)
	if len(ids) != 1 || stockDos.Cantidad != 7 || stockDos.Version != 1 {
		t.Fatalf("la clave concurrente duplicó efectos: ids=%d stock=%+v", len(ids), stockDos)
	}
	fmt.Fprintf(&reporte, "- Idempotencia concurrente: 8 respuestas 201; movimientos únicos=%d, saldo=%d, versión=%d.\n", len(ids), stockDos.Cantidad, stockDos.Version)
	e.modoCatalogo.Store(503)
	consultasAntes := e.consultasCatalogo.Load()
	repetida := e.pedir(t, "POST", "/api/v1/movimientos", e.tokenAdmin, claveIdempotente, cuerpoIdempotente, 201)
	if !bytes.Equal(original.Cuerpo, repetida.Cuerpo) {
		t.Fatal("el replay cambió el movimiento confirmado")
	}
	e.pedir(t, "POST", "/api/v1/movimientos", e.tokenAdmin, claveIdempotente, cuerpoMovimiento(productoDos, "entrada", 8, nil), 409)
	e.pedir(t, "POST", "/api/v1/movimientos", e.tokenAdmin, claveIdempotente, cuerpoMovimiento(productoTres, "entrada", 7, nil), 409)
	diferenciaConsultas := e.consultasCatalogo.Load() - consultasAntes
	if diferenciaConsultas != 0 {
		t.Fatal("replay y conflicto de clave confirmada no deben depender de Productos")
	}
	e.modoCatalogo.Store(0)
	fmt.Fprintf(&reporte, "- Replay con Productos caído: mismo movimiento; consultas proveedor=%d; conflicto de clave=409.\n", diferenciaConsultas)

	version := int64(2)
	ajuste := decodificar[movimientoAPI](t, e.pedir(t, "POST", "/api/v1/movimientos", e.tokenAdmin, "ajuste-a-cinco-001", cuerpoMovimiento(productoUno, "ajuste", 5, &version), 201))
	if ajuste.Cantidad != 5 || ajuste.SaldoPosterior != 5 {
		t.Fatalf("el ajuste no tomó la cantidad como saldo objetivo: %+v", ajuste)
	}
	e.pedir(t, "POST", "/api/v1/movimientos", e.tokenAdmin, "ajuste-obsoleto-001", cuerpoMovimiento(productoUno, "ajuste", 8, &version), 409)
	e.pedir(t, "POST", "/api/v1/movimientos", e.tokenAdmin, "ajuste-sin-version", cuerpoMovimiento(productoUno, "ajuste", 8, nil), 422)
	version = 3
	e.pedir(t, "POST", "/api/v1/movimientos", e.tokenAdmin, "ajuste-sin-cambio-01", cuerpoMovimiento(productoUno, "ajuste", 5, &version), 422)
	ajuste = decodificar[movimientoAPI](t, e.pedir(t, "POST", "/api/v1/movimientos", e.tokenAdmin, "ajuste-a-cero-001", cuerpoMovimiento(productoUno, "ajuste", 0, &version), 201))
	if ajuste.Cantidad != 5 || ajuste.SaldoAnterior != 5 || ajuste.SaldoPosterior != 0 {
		t.Fatalf("el ajuste a cero perdió la variación: %+v", ajuste)
	}
	stock = consultarStock(t, e, productoUno)
	if stock.Cantidad != 0 || stock.Version != 4 {
		t.Fatalf("saldo tras ajustes: %+v", stock)
	}
	fmt.Fprintf(&reporte, "- Ajustes: objetivos 5 y 0; versión obsoleta=409; versión ausente=422; sin variación=422; saldo=%d, versión=%d.\n", stock.Cantidad, stock.Version)

	e.pedir(t, "POST", "/api/v1/movimientos", e.tokenAdmin, "entrada-maxima-001", cuerpoMovimiento(productoTres, "entrada", 1_000_000_000, nil), 201)
	e.pedir(t, "POST", "/api/v1/movimientos", e.tokenAdmin, "entrada-supera-maximo", cuerpoMovimiento(productoTres, "entrada", 1, nil), 422)
	stockTres := consultarStock(t, e, productoTres)
	if stockTres.Cantidad != 1_000_000_000 || stockTres.Version != 1 {
		t.Fatalf("se excedió capacidad máxima: %+v", stockTres)
	}
	fmt.Fprintf(&reporte, "- Límite de cantidad: saldo máximo=%d; desborde=422; versión=%d.\n", stockTres.Cantidad, stockTres.Version)

	invalidas := [][]byte{
		cuerpoMovimiento(productoUno, "entrada", -1, nil), cuerpoMovimiento(productoUno, "salida", 0, nil),
		cuerpoMovimiento(productoUno, "entrada", 1_000_000_001, nil), cuerpoMovimiento("no-es-uuid", "entrada", 1, nil),
		cuerpoMovimiento(productoUno, "transferencia", 1, nil),
		[]byte(`{"producto_id":"` + productoUno + `","tipo":"entrada","cantidad":1.5,"motivo":"Prueba"}`),
		[]byte(`{"producto_id":"` + productoUno + `","tipo":"entrada","cantidad":1}`),
		[]byte(`{"producto_id":"` + productoUno + `","tipo":"entrada","cantidad":1,"motivo":"Prueba","campo_desconocido":true}`),
	}
	for i, cuerpo := range invalidas {
		e.pedir(t, "POST", "/api/v1/movimientos", e.tokenAdmin, fmt.Sprintf("solicitud-invalida-%02d", i), cuerpo, 422)
	}
	fmt.Fprintf(&reporte, "- Contrato: %d solicitudes inválidas rechazadas con 422 y errores RFC 9457 sin detalles internos.\n", len(invalidas))

	e.pedir(t, "POST", "/api/v1/movimientos", e.tokenAdmin, "producto-ausente-001", cuerpoMovimiento(productoAusente, "entrada", 1, nil), 404)
	e.modoCatalogo.Store(503)
	e.pedir(t, "POST", "/api/v1/movimientos", e.tokenAdmin, "proveedor-caido-001", cuerpoMovimiento(productoAusente, "entrada", 1, nil), 503)
	e.modoCatalogo.Store(-2)
	e.pedir(t, "POST", "/api/v1/movimientos", e.tokenAdmin, "proveedor-json-invalido", cuerpoMovimiento(productoAusente, "entrada", 1, nil), 503)
	e.modoCatalogo.Store(-1)
	inicio := time.Now()
	e.pedir(t, "POST", "/api/v1/movimientos", e.tokenAdmin, "proveedor-timeout-001", cuerpoMovimiento(productoAusente, "entrada", 1, nil), 503)
	if time.Since(inicio) > 8*time.Second {
		t.Fatal("la dependencia excedió el timeout HTTP de cinco segundos")
	}
	e.modoCatalogo.Store(0)
	var nuevas int
	if err := e.db.QueryRow("SELECT count(*) FROM existencias WHERE producto_id=$1::uuid", productoAusente).Scan(&nuevas); err != nil {
		t.Fatal(err)
	}
	if nuevas != 0 {
		t.Fatal("un fallo de Productos persistió stock")
	}
	fmt.Fprintf(&reporte, "- Productos: ausente=404; caído=503; JSON inválido=503; timeout=503; existencias nuevas=%d.\n", nuevas)

	ajustes := decodificar[paginaAPI[movimientoAPI]](t, e.pedir(t, "GET", "/api/v1/movimientos?producto_id="+productoUno+"&tipo=ajuste&limite=1&desplazamiento=1", e.tokenAdmin, "", nil, 200))
	if ajustes.Total != 2 || len(ajustes.Datos) != 1 || ajustes.Datos[0].Tipo != "ajuste" {
		t.Fatalf("filtro o paginación de historial incorrectos: %+v", ajustes)
	}
	vacia := decodificar[paginaAPI[movimientoAPI]](t, e.pedir(t, "GET", "/api/v1/movimientos?desplazamiento=100", e.tokenAdmin, "", nil, 200))
	if vacia.Total != 6 || vacia.Datos == nil || len(vacia.Datos) != 0 {
		t.Fatalf("la página vacía perdió el total: %+v", vacia)
	}
	pagina := decodificar[paginaAPI[existenciaAPI]](t, e.pedir(t, "GET", "/api/v1/existencias?limite=1&desplazamiento=1", e.tokenAdmin, "", nil, 200))
	if pagina.Total != 3 || len(pagina.Datos) != 1 {
		t.Fatalf("paginación de existencias: %+v", pagina)
	}
	todo := decodificar[paginaAPI[movimientoAPI]](t, e.pedir(t, "GET", "/api/v1/movimientos", e.tokenAdmin, "", nil, 200))
	for _, m := range todo.Datos {
		if m.CreadoEn.IsZero() || m.CreadoEn.Location() != time.UTC {
			t.Fatalf("fecha sin UTC/RFC3339: %+v", m)
		}
	}
	fmt.Fprintf(&reporte, "- Consultas: ajustes filtrados=%d; página de ajustes=1; página vacía=[] con total=%d; existencias=%d.\n", ajustes.Total, vacia.Total, pagina.Total)

	var movimientos, auditoria, inconsistencias int64
	if err := e.db.QueryRow("SELECT count(*) FROM movimientos").Scan(&movimientos); err != nil {
		t.Fatal(err)
	}
	if err := e.db.QueryRow("SELECT count(*) FROM auditoria").Scan(&auditoria); err != nil {
		t.Fatal(err)
	}
	if err := e.db.QueryRow(`SELECT count(*) FROM existencias e WHERE e.version <> (SELECT count(*) FROM movimientos m WHERE m.producto_id=e.producto_id)
OR e.cantidad <> (SELECT saldo_posterior FROM movimientos m WHERE m.producto_id=e.producto_id ORDER BY creado_en DESC,id DESC LIMIT 1)`).Scan(&inconsistencias); err != nil {
		t.Fatal(err)
	}
	var auditoriasInvalidas int64
	if err := e.db.QueryRow(`SELECT count(*) FROM auditoria a JOIN movimientos m ON m.id=a.movimiento_id
WHERE a.actor_id IS DISTINCT FROM m.actor_id OR a.accion IS DISTINCT FROM m.tipo
OR a.creado_en IS DISTINCT FROM m.creado_en
OR a.datos->>'producto_id' IS DISTINCT FROM m.producto_id::text
OR a.datos->>'saldo_posterior' IS DISTINCT FROM m.saldo_posterior::text`).Scan(&auditoriasInvalidas); err != nil {
		t.Fatal(err)
	}
	inconsistencias += auditoriasInvalidas
	if movimientos != 6 || auditoria != movimientos || inconsistencias != 0 {
		t.Fatalf("atomicidad de stock/movimiento/auditoría: %d/%d; inconsistencias=%d", movimientos, auditoria, inconsistencias)
	}
	fmt.Fprintf(&reporte, "- Atomicidad: movimientos=%d; auditorías=%d; inconsistencias=%d.\n", movimientos, auditoria, inconsistencias)
	if e.cabecerasInvalidas.Load() != 0 {
		t.Fatal("no se propagaron JWT y X-Request-Id en todas las llamadas a Productos")
	}
	reporte.WriteString("- Correlación: JWT y X-Request-Id propagados; fechas RFC 3339 en UTC.\n")
	verificarCaidaBase(t, e)
	reporte.WriteString("- PostgreSQL no disponible: readyz=503; healthz=200; recuperación readyz=200.\n")
	golden := filepath.Join("testdata", "flujo.golden.md")
	esperado, err := os.ReadFile(golden)
	if err != nil {
		t.Fatal(err)
	}
	if reporte.String() != string(esperado) {
		t.Fatalf("reporte distinto de %s\nRECIBIDO:\n%s\nESPERADO:\n%s", golden, reporte.String(), esperado)
	}
}

func consultarStock(t *testing.T, e *escenario, producto string) existenciaAPI {
	t.Helper()
	return decodificar[existenciaAPI](t, e.pedir(t, "GET", "/api/v1/existencias/"+producto, e.tokenAdmin, "", nil, 200))
}

func enParalelo(t *testing.T, cantidad int, operacion func(int) (respuesta, error)) []respuesta {
	t.Helper()
	type resultado struct {
		respuesta respuesta
		err       error
	}
	inicio := make(chan struct{})
	resultados := make(chan resultado, cantidad)
	for i := 0; i < cantidad; i++ {
		go func(indice int) { <-inicio; r, err := operacion(indice); resultados <- resultado{r, err} }(i)
	}
	close(inicio)
	respuestas := make([]respuesta, 0, cantidad)
	for i := 0; i < cantidad; i++ {
		resultado := <-resultados
		if resultado.err != nil {
			t.Fatalf("operación concurrente: %v", resultado.err)
		}
		respuestas = append(respuestas, resultado.respuesta)
	}
	return respuestas
}

func verificarCaidaBase(t *testing.T, e *escenario) {
	t.Helper()
	nombre := pgx.Identifier{e.nombreDB}.Sanitize()
	t.Cleanup(func() { _, _ = e.admin.Exec("ALTER DATABASE " + nombre + " ALLOW_CONNECTIONS true") })
	if _, err := e.admin.Exec("ALTER DATABASE " + nombre + " ALLOW_CONNECTIONS false"); err != nil {
		t.Fatal(err)
	}
	if _, err := e.admin.Exec("SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname=$1 AND pid<>pg_backend_pid()", e.nombreDB); err != nil {
		t.Fatal(err)
	}
	e.pedir(t, "GET", "/readyz", "", "", nil, 503)
	e.pedir(t, "GET", "/healthz", "", "", nil, 200)
	if _, err := e.admin.Exec("ALTER DATABASE " + nombre + " ALLOW_CONNECTIONS true"); err != nil {
		t.Fatal(err)
	}
	ctx, cancelar := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelar()
	for {
		r, err := e.solicitar("GET", "/readyz", "", "", nil, false, "")
		if err == nil && r.Estado == 200 {
			return
		}
		select {
		case <-ctx.Done():
			t.Fatal("PostgreSQL no recuperó readiness")
		case <-time.After(50 * time.Millisecond):
		}
	}
}
