# STOCK · Grupo 8

Microservicio de inventario en Go y PostgreSQL, acompañado de microcomponentes React
reutilizables. Registra entradas, salidas y ajustes; consulta existencias e historial;
protege los saldos con transacciones, idempotencia y control de versión.

El catálogo pertenece a **Productos (grupo 3)**. STOCK conserva identificadores de
producto, cantidades, movimientos y auditoría. Los documentos recibidos están en
`modulo/` y se mantienen como referencia.

## Alcance y decisiones pendientes

La división de grupos sitúa STOCK como el segundo servicio. Se usan provisionalmente
los puertos **5120 web** y **5121 API** conforme a `51N0` / `51N1` de DAS; grupo 8 no
equivale al número de servicio. Falta el documento oficial de componentes para confirmar
esta asignación y los contratos finales de Productos e Identidad.

`contracts/externos/productos.yaml` es una propuesta mínima para avanzar, **no un
contrato entregado por el grupo proveedor**. El namespace `pingeso` del módulo Go y del
paquete npm también debe confirmarse antes de publicar. La integración y las decisiones
están descritas en [docs/integracion.md](docs/integracion.md).

## Requisitos

- Go **1.26.11** (actualización de seguridad sobre 1.26.5 de DAS para mitigar CVE de `stdlib`), Node.js **22.22.0** y pnpm **10.13.1**.
- Docker Engine/Desktop con Docker Compose y contenedores Linux; necesario para
  PostgreSQL real, las pruebas y los builds reproducibles.
- Gitleaks **8.30.1** para el hook local. GNU Make es opcional: los comandos equivalentes
  están en `Makefile` y los scripts de pnpm funcionan sin Make.

Las versiones corresponden exactamente a DAS. Los contenedores incluyen los toolchains
de compilación; los runtimes API y web son distroless y se ejecutan sin root.

## Instalación y demostración de interfaz

```sh
corepack enable
corepack prepare pnpm@10.13.1 --activate
pnpm install --frozen-lockfile
pnpm exec lefthook install
pnpm codegen
pnpm dev
```

Abrir `http://localhost:5120/?demo=1` activa explícitamente los mocks MSW de la app de
desarrollo. Permite recorrer existencias, movimientos e historial sin credenciales ni
servicios externos. Los datos son ficticios y viven en memoria del navegador. Abrir la
app sin `?demo=1` utiliza la API real y requiere una sesión válida.

## Ejecución del sistema con PostgreSQL

Copiar `.env.example` a `.env` (`Copy-Item .env.example .env` en PowerShell o
`cp .env.example .env` en una terminal POSIX). Completar todos los valores
`replace-with-*`, usando las URLs y credenciales del ambiente de integración:

```sh
docker compose config --quiet
docker compose up -d --build
docker compose ps
```

El orden es PostgreSQL saludable → migraciones completadas → API saludable → web.
La API no migra al iniciar. Una migración fallida impide arrancarla.

| Dirección                        | Uso                                       |
| -------------------------------- | ----------------------------------------- |
| `http://localhost:5120`          | Aplicación y proxy `/api` a la API        |
| `http://127.0.0.1:5121/healthz`  | Salud del proceso API                     |
| `http://127.0.0.1:5121/readyz`   | Conectividad con PostgreSQL               |
| `http://127.0.0.1:5121/api/docs` | Documentación interactiva en `dev`/`test` |
| `http://localhost:8025`          | Mailpit, sólo perfil `dev`                |

El perfil `dev` carga una existencia inicial en cero para el UUID
`10000000-0000-4000-8000-000000000001`. No crea el producto en un servicio ajeno: el
proveedor debe tener ese identificador para permitir movimientos reales. STOCK no envía
correo; Mailpit forma parte del entorno base solicitado.

La API valida firma RS256/EdDSA, emisor, audiencia y vencimiento del JWT del proveedor
Identidad. Si el JWKS no está disponible, el arranque falla. Completar `.env` es necesario
para operar contra servicios reales. El entorno de demostración de la interfaz es
independiente de esa configuración.

`docker compose down` detiene los servicios y conserva la base en el volumen nombrado.
Los puertos se publican sólo en la interfaz local. PostgreSQL no publica un puerto al host.

## Contrato y operación

El contrato generado es [contracts/openapi.yaml](contracts/openapi.yaml), OpenAPI 3.1.
La documentación y el spec HTTP se deshabilitan con `API_ENV=prod`.

| Método | Ruta                                 | Permiso             |
| ------ | ------------------------------------ | ------------------- |
| GET    | `/api/v1/existencias`                | `stock:consultar`   |
| GET    | `/api/v1/existencias/{producto_id}`  | `stock:consultar`   |
| GET    | `/api/v1/movimientos`                | `stock:consultar`   |
| POST   | `/api/v1/movimientos` entrada/salida | `stock:operar`      |
| POST   | `/api/v1/movimientos` ajuste         | `stock:administrar` |

Todo movimiento requiere `Idempotency-Key`. Una clave se mantiene al reintentar la misma
operación. Cambiar el contenido con la misma clave devuelve 409. Un ajuste recibe el saldo
final y `version_esperada`; una versión obsoleta devuelve 409. Las salidas que exceden el
saldo también devuelven 409. Los errores tienen formato `application/problem+json`.

Ejemplo desde un cliente de servicio que ya obtuvo un JWT legítimo:

```sh
curl http://127.0.0.1:5121/api/v1/movimientos \
  -H "Authorization: Bearer $JWT" \
  -H 'Content-Type: application/json' \
  -H 'Idempotency-Key: compra-2026-001' \
  -d '{"producto_id":"10000000-0000-4000-8000-000000000001","tipo":"entrada","cantidad":10,"motivo":"Recepción de compra","referencia":"OC-001"}'
```

En el navegador la sesión viaja en cookie `HttpOnly`, `Secure`, `SameSite=Lax` emitida por
Identidad. STOCK valida esa cookie y no emite sesiones ni almacena tokens en el navegador.
Las escrituras con cookie requieren un `Origin` igual a `FRONTEND_URL`. El proxy web
mantiene un solo origen y reenvía las cookies.

## Microcomponentes

El paquete `@pingeso/stock-ui` exporta `ConsultaStock`, `MovimientoStock` e
`HistorialStock`. Cada componente recibe `apiBaseUrl` y `permisos`; no depende del router
ni del estado de la aplicación anfitriona. Los permisos visuales no sustituyen la
autorización del servidor.

```tsx
import { ConsultaStock, MovimientoStock, HistorialStock } from '@pingeso/stock-ui'
import '@pingeso/stock-ui/estilos.css'

const permisos = ['stock:consultar', 'stock:operar']

export function Inventario() {
  return (
    <>
      <ConsultaStock apiBaseUrl="" permisos={permisos} />
      <MovimientoStock apiBaseUrl="" permisos={permisos} />
      <HistorialStock apiBaseUrl="" permisos={permisos} />
    </>
  )
}
```

La cadena vacía usa el mismo origen. La aplicación anfitriona debe ofrecer el proxy y la
sesión de Identidad. `pnpm build` compila la app en `apps/web/dist` y el paquete en
`apps/web/dist-paquete`. La app de desarrollo y sus mocks no se incluyen en el paquete.

## Generación y verificación

```sh
pnpm codegen
make generate
make clients
pnpm verify
pnpm test:e2e:api
```

`codegen` regenera OpenAPI y TypeScript; `make generate` usa sqlc **v1.29.0** y
`make clients` usa oapi-codegen **v2.8.2**. No editar los archivos generados. Tras
regenerar, `git diff --exit-code` debe quedar limpio en una revisión del mismo contrato.

El E2E ejecuta un binario Go real compilado con cobertura contra PostgreSQL real.
Testcontainers requiere Docker disponible. También puede proporcionarse
`TEST_DATABASE_URL` con una base exclusiva de pruebas. El runner produce `coverage.out`
y compara un reporte determinista con el golden de `apps/api/e2e/testdata/`.
Las pruebas del frontend generan `apps/web/coverage/lcov.info`.

Los escenarios y su prueba de sensibilidad están descritos en
[docs/modos-de-falla.md](docs/modos-de-falla.md). CI repite el E2E en `TZ=UTC` y
`TZ=Pacific/Auckland`. Para ejecutarlo manualmente, establecer `TZ` en la terminal antes
del comando; en PowerShell: `$env:TZ='UTC'; pnpm test:e2e:api`.

## CI, seguridad y publicación

[.github/workflows/ci.yml](.github/workflows/ci.yml) incluye los checks `calidad`,
`seguridad` e `imagen`: formato, límites de componentes, tipos, build, E2E, contrato,
SonarQube y Quality Gate, Gitleaks, govulncheck, audit, Hadolint, Trivy y Dockle.
No se deshabilita SonarQube cuando faltan secretos: deben configurarse para completar CI.

La publicación es una ejecución manual de `ci` en `main` con `publicar=true`. El mismo job
construye, verifica y promueve las imágenes API, migraciones y web; publica el paquete,
firma imágenes con Cosign y adjunta contrato, SBOM CycloneDX y notas a la release.
No se ha publicado ningún artefacto desde este entorno.

Antes de la entrega, el responsable del repositorio debe configurar:

- Proyecto SonarCloud/SonarQube (`LucianoXHDX_stock_pingeso`, organización `lucianoxhdx`), definición de código nuevo y Quality Gate DAS; secrets `SONAR_TOKEN` y `SONAR_HOST_URL` (`https://sonarcloud.io`).
- Protección de `main`: PR, una aprobación y checks `calidad`, `seguridad`, `imagen`.
- Namespace definitivo y permisos de GHCR/GitHub Packages; versión npm nueva por release.
- Contratos oficiales y conectividad con Productos e Identidad.
- Enlaces reales al pipeline y al proyecto SonarQube como evidencia del Hito 0.

El estado verificable y las tareas externas se registran en
[docs/cumplimiento.md](docs/cumplimiento.md). La presencia de los workflows no representa
un pipeline ni un Quality Gate ejecutados con éxito.

La integración unificada actualiza el toolchain e imágenes base de Go a **1.26.11**
(digest fijado `sha256:23f198931d4bd83b4f5a613bd75656257ae8bf832dcbc2db80e1050b5219ca2a`)
para resolver los hallazgos de `stdlib 1.26.5`. Persisten bloqueos externos pendientes de
coordinación en Vite 8.1.5, Faker transitivo de Prism y el módulo independiente de goose
3.24.1; véase [docs/verificacion.md](docs/verificacion.md). El pipeline mantiene todos los
controles activos.
