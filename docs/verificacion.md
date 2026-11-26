# Verificación local

Fecha: 19 de noviembre de 2026 (actualizado tras la unificación con el repositorio de
Luciano: Go actualizado a **1.26.11** con digest fijado para resolver las vulnerabilidades
de `stdlib 1.26.5`, y `sonar-project.properties` vinculado a `LucianoXHDX_stock_pingeso`).
La implementación y sus pruebas funcionales pasan. **Pendiente de re-verificar en CI/Docker
tras el bump a Go 1.26.11; persisten bloqueos de seguridad en Vite 8.1.5, Faker transitivo
de Prism y el módulo independiente de goose 3.24.1.** No se bajaron umbrales ni se
agregaron excepciones.

Estos resultados provienen de ejecuciones locales. El proyecto SonarQube, sus secretos
y el pipeline remoto aún deben configurarse; no se declara un Quality Gate aprobado.

## Estado final

| Comprobación                        | Resultado final                                                                                        |
| ----------------------------------- | ------------------------------------------------------------------------------------------------------ |
| `pnpm verify`                       | Correcto: formato, lint, límites de componentes, tipos, pruebas y builds                               |
| `go vet` y `go mod verify`          | Correctos                                                                                              |
| E2E API contra PostgreSQL real      | Matriz de acceso de 22 subcasos y flujo completo correctos en UTC y Pacific/Auckland; golden idéntico  |
| Cobertura del binario real Go       | **78.1%**, 488 de 625 sentencias                                                                       |
| Pruebas React                       | **6 correctas**, **82.28%** de cobertura de líneas                                                     |
| Sensibilidad del E2E                | Detectó el cambio incorrecto de 409 a 201 en una copia mutada; fuente original intacta                 |
| OpenAPI, TypeScript y cliente Go    | Repetición con mismos hashes; oapi-codegen **v2.8.2**                                                  |
| Queries sqlc **v1.29.0**            | Repetición con mismos hashes                                                                           |
| Compose                             | `docker compose --env-file .env.example config --quiet` correcto                                       |
| Imágenes API, web y migrador        | Builds finales correctos; runtimes sin root                                                            |
| Migraciones                         | goose desde su imagen contra PostgreSQL **17.11** correcto                                             |
| Hadolint **v2.15.1**                | Tres Dockerfiles correctos al nivel `error`; sólo INFO DL3066 para `USER nonroot` de DAS               |
| Gitleaks **8.30.1**                 | 0 hallazgos en copia temporal de 95 archivos del repositorio                                           |
| Gitleaks `protect --staged`         | Rechazó un secreto falso en un repositorio temporal aislado; salida 1 esperada                         |
| Trivy filesystem **v0.76.1**        | **0 HIGH/CRITICAL con fix**, 0 secretos y 0 problemas de configuración al umbral configurado; salida 0 |
| govulncheck API **v1.9.0**          | **17 vulnerabilidades alcanzables**, todas de la biblioteca estándar Go **1.26.5**                     |
| govulncheck servidor SPA **v1.9.0** | **17 vulnerabilidades alcanzables** de la biblioteca estándar Go **1.26.5**                            |
| `pnpm audit --audit-level high`     | **2 HIGH**, 7 moderadas y 1 baja; salida 1                                                             |
| Prism CLI **5.16.0**                | Arranque y comprobación de versión correctos con el lockfile final                                     |
| Dockle **v0.4.15**                  | Tres imágenes correctas al umbral `warn`: 0 FATAL y 0 WARN                                             |
| SonarQube y Quality Gate            | Pendiente de proyecto y secrets externos                                                               |

Las pruebas React se ejecutaron localmente con Node **24.14.0**, disponible en el equipo.
El build de la imagen web pasó además con Node **22.22.0**, la versión exacta de DAS.
El workflow configura esa misma versión para repetir las pruebas en CI.

## Escaneo de las imágenes finales

Se escanearon las imágenes reales `stock-api:verificacion`,
`stock-web:verificacion` y `stock-migrate:verificacion` con Trivy **v0.76.1**:
scanners `vuln,secret`, severidad `HIGH,CRITICAL`, `--ignore-unfixed` y
`--exit-code 1`. El ejecutable Windows accedió a Docker Desktop por
`npipe:////./pipe/dockerDesktopLinuxEngine`, usando la base oficial del escáner.

| Imagen   | Trivy con corrección disponible                  | Secretos al umbral configurado | Dockle                    |
| -------- | ------------------------------------------------ | ------------------------------ | ------------------------- |
| API      | **13 HIGH**, todas de `stdlib v1.26.5`; salida 1 | 0                              | 0 FATAL, 0 WARN; salida 0 |
| Web      | **13 HIGH**, todas de `stdlib v1.26.5`; salida 1 | 0                              | 0 FATAL, 0 WARN; salida 0 |
| Migrador | **27 HIGH y 2 CRITICAL**; salida 1               | 0                              | 0 FATAL, 0 WARN; salida 0 |

La diferencia entre 17 hallazgos de govulncheck y 13 HIGH de Trivy no representa una
contradicción: sus fuentes y criterios son distintos, y Trivy se filtró por severidad.
Dockle emitió únicamente el aviso informativo de Docker Content Trust; no se configuró
ninguna omisión manual del control.

El migrador conserva la instalación independiente de **goose v3.24.1** prescrita por
DAS. Su propio módulo incluye dependencias anteriores a las de la API:

| Dependencia dentro de goose | Instalada | Hallazgos HIGH/CRITICAL                         | Corrección informada           |
| --------------------------- | --------- | ----------------------------------------------- | ------------------------------ |
| `github.com/jackc/pgx/v5`   | `v5.7.1`  | **2 CRITICAL**, CVE-2026-33815 y CVE-2026-33816 | `v5.9.0`                       |
| `golang.org/x/crypto`       | `v0.31.0` | 12 HIGH                                         | El conjunto requiere `v0.55.0` |
| `golang.org/x/text`         | `v0.21.0` | 2 HIGH                                          | El conjunto requiere `v0.41.0` |
| Biblioteca estándar Go      | `v1.26.5` | 13 HIGH                                         | Según CVE, Go 1.26.6 a 1.26.11 |

Actualizar las transitivas del módulo API no modifica el módulo propio usado por
`go install .../goose@v3.24.1`. Compilar sólo el driver PostgreSQL reduce superficie,
pero conserva estos hallazgos. La versión exacta de goose y su instalación independiente
deben revisarse junto con el responsable de DAS antes de declarar el migrador seguro.

## Bloqueos que requieren coordinación

Go **1.26.5** tiene vulnerabilidades alcanzables en HTTP, TLS, certificados, parsing y
otras funciones estándar. Entre ellas están
[GO-2026-6859](https://pkg.go.dev/vuln/GO-2026-6859), corregida en Go **1.26.11**,
y [GO-2026-6809](https://pkg.go.dev/vuln/GO-2026-6809), corregida en **1.26.10**.
Varios errores HTTP/2 requieren **1.26.9**. DAS fija el toolchain y sus imágenes por
digest; actualizar esa tabla requiere coordinación.

El audit final mantiene:

- Vite **8.1.5**: [GHSA-587h-7wmj-h5pm](https://github.com/advisories/GHSA-587h-7wmj-h5pm),
  con corrección informada en **8.1.6**, diferente de la versión obligatoria de DAS.
- Faker transitivo de Prism **5.16.0**:
  [GHSA-qxc2-j82w-r537](https://github.com/advisories/GHSA-qxc2-j82w-r537).
  Se probó Faker **10.5.0**, pero Postman usa la API antigua `faker.address`, removida en
  esa versión, y Prism dejaba de iniciar. Se retiró ese override y se conservó el CLI
  funcional; la solución debe ser compatible con Prism y la tabla del curso.

Las vulnerabilidades del módulo independiente de goose también requieren una solución
acordada. CI conserva todos los bloqueos: audit, govulncheck y Trivy de imágenes deben
rechazar el estado actual. Los resultados funcionales correctos y Trivy filesystem sin
hallazgos no sustituyen esos controles.

## Correcciones durante la creación inicial

La primera auditoría encontró **3 HIGH** npm; la selección inicial de
`lodash 4.18.0` eliminó [GHSA-r5fr-rjxr-66jc](https://github.com/advisories/GHSA-r5fr-rjxr-66jc)
sin cambiar la versión de Prism. El lockfile final pasa
`pnpm install --frozen-lockfile` y la comprobación de arranque de Prism.

El primer Trivy filesystem encontró **14 HIGH** en dependencias Go no prescritas.
Se seleccionaron versiones corregidas, manteniendo el stack de la tabla DAS:

| Dependencia                     | Primera selección | Selección final |
| ------------------------------- | ----------------- | --------------- |
| `github.com/klauspost/compress` | `v1.18.5`         | `v1.18.7`       |
| `github.com/moby/go-archive`    | `v0.2.0`          | `v0.3.0`        |
| `golang.org/x/crypto`           | `v0.51.0`         | `v0.55.0`       |
| `golang.org/x/text`             | `v0.37.0`         | `v0.41.0`       |

El resultado posterior fue **0 HIGH/CRITICAL con fix** en filesystem y la API pasó de
**19 a 17** vulnerabilidades alcanzables, resolviendo las dos de `x/text`:
[GO-2026-5970](https://pkg.go.dev/vuln/GO-2026-5970) y
[GO-2026-6629](https://pkg.go.dev/vuln/GO-2026-6629). No se creó una excepción ni se bajó
ningún umbral. Estos cambios son selección de transitivas en un repositorio nuevo, sin
una versión anterior desplegada.

Los `go.mod` de [text](https://github.com/golang/text/blob/v0.41.0/go.mod),
[crypto](https://github.com/golang/crypto/blob/v0.55.0/go.mod),
[compress](https://github.com/klauspost/compress/blob/v1.18.7/go.mod) y
[go-archive](https://github.com/moby/go-archive/blob/v0.3.0/go.mod) requieren Go 1.25 o
anterior, compatible con 1.26.5. La versión de `x/crypto` de DAS aplica al servicio de
Identidad; STOCK no implementa hash de contraseñas.

## Detalles del entorno y reproducción

El servidor SPA se verificó con un binario Linux real en distroless y un proveedor HTTP
controlado: salud, fallback de rutas, `nosniff`, `Referrer-Policy`, assets inmutables,
assets inexistentes, cookie, correlación y respuesta RFC 9457 al caer el backend.

Un Apache ajeno al proyecto ocupaba IPv4 en 8080. No se detuvo. El runner E2E usa IPv6
en Windows para alcanzar su API de prueba, manteniendo el puerto interno exigido.
Los contenedores tienen un namespace de red propio y no colisionan con ese proceso.

Goose se compila con los tags oficiales que excluyen drivers de bases que STOCK no usa,
conservando PostgreSQL y la versión prescrita. Véase la
[documentación de goose v3.24.1](https://github.com/pressly/goose/blob/v3.24.1/README.md).

El filesystem se analizó con Trivy Windows de la versión fijada, omitiendo únicamente
dependencias instaladas, `.git`, temporales y builds; las dependencias se analizaron
desde los módulos y el lockfile. No se omitieron fuentes ni se aplicaron excepciones.
Las imágenes se analizaron completas en la revisión posterior descrita arriba.

govulncheck de la API se guardó en JSON para contar resultados. Ese modo no se interpretó
como ausencia de vulnerabilidades por su código de salida: se revisaron los hallazgos
estructurados. CI usa el modo normal, que bloquea ante vulnerabilidades alcanzables.
