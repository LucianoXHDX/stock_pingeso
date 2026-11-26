# Modos de falla de STOCK

Definido antes de implementar, conforme a DAS. El grupo 8 administra cantidades y
movimientos; Productos (grupo 3) es dueño del catálogo. No se comparte su base.

| Falla                                           | Protección prevista                                                          | Verificación                                           |
| ----------------------------------------------- | ---------------------------------------------------------------------------- | ------------------------------------------------------ |
| Dos salidas consumen la última unidad           | Transacción y bloqueo de la existencia; CHECK cantidad >= 0                  | E2E con solicitudes simultáneas: una 201 y otra 409    |
| Un reintento duplica un movimiento              | Clave de idempotencia por usuario, hash de solicitud y transacción atómica   | E2E repite clave y comprueba movimiento y saldo únicos |
| Misma clave con otro contenido                  | Comparación de huella, incluido tipo y producto                              | E2E espera 409 y saldo intacto                         |
| Un ajuste pisa cambios concurrentes             | Versión esperada obligatoria; control optimista dentro del bloqueo           | E2E ajuste con versión anterior devuelve 409           |
| Cantidad negativa, fraccionaria o excesiva      | Validación Huma, reglas de dominio y constraints PostgreSQL                  | E2E entradas inválidas devuelve 422                    |
| Producto inexistente                            | Consulta REST a Productos antes de crear un movimiento                       | E2E proveedor 404, sin cambios locales                 |
| Productos no disponible o lento                 | Timeout HTTP de 5 segundos, 503 sin confirmar operación                      | E2E proveedor no disponible y saldo intacto            |
| Acceso sin sesión o sin permiso                 | JWT RS256/EdDSA validado por JWKS, issuer y audience; permisos por operación | Matriz de roles en acceso_test.go y E2E 401/403        |
| Escritura desde un origen ajeno usando cookie   | Comprobar Origin para solicitudes que mutan                                  | E2E origen no permitido devuelve 403                   |
| Movimiento sin auditoría o saldo sin movimiento | Una transacción guarda saldo, movimiento, auditoría e idempotencia           | E2E consulta historial, saldo y auditoría SQL          |
| Filtración de SQL, tokens o datos internos      | Errores RFC 9457 y logs estructurados sin cuerpos ni credenciales            | E2E revisa errores y revisión de middleware            |
| Cambio accidental de contrato                   | OpenAPI y TypeScript generados, diff en CI y oasdiff                         | pnpm codegen y control de compatibilidad               |
| PostgreSQL caído                                | /readyz consulta la base con timeout                                         | E2E y healthcheck; /healthz sigue independiente        |
| Cambios por huso horario                        | timestamptz y fechas RFC 3339                                                | Mismo E2E en UTC y Pacific/Auckland                    |

La prueba de sensibilidad debe desactivar temporalmente la protección de
concurrencia en una copia del código, ejecutar el E2E, comprobar que falla y
restaurar la protección. No se integra código con protecciones desactivadas.

## Ejecución y sensibilidad

`pnpm test:e2e:api` compila el binario instrumentado, ejecuta las pruebas reales y
genera `coverage.out`. La matriz de acceso está en `apps/api/e2e/acceso_test.go` y
el reporte determinista en `apps/api/e2e/testdata/flujo.golden.md`.

`node scripts/test-sensibilidad.mjs` crea una copia temporal de la API, elimina en
esa copia la comparación de versión de los ajustes y ejecuta `TestFlujoStock` con
el binario modificado. Sólo acepta como evidencia que un ajuste obsoleto devuelva
201 y que el E2E falle esperando 409. Una falla de compilación o infraestructura
no cuenta como detección. El código del proyecto conserva su protección.

Las pruebas usan bases efímeras propias. Si se proporciona `TEST_DATABASE_URL`,
su usuario debe tener permiso para crear y eliminar bases de prueba. Nunca se
debe apuntar a un servidor productivo. En Windows se usa el loopback IPv6
`[::1]:8080` para evitar la colisión con servicios que ya ocupen IPv4; el puerto
interno de los contenedores sigue siendo 8080. `STOCK_TEST_API_URL` permite
especificar explícitamente la dirección local del binario de prueba.
