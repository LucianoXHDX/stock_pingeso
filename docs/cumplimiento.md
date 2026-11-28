# Evidencia de cumplimiento de DAS

Este documento distingue artefactos implementados de verificaciones y decisiones que
requieren el repositorio remoto, los proveedores o al jefe de proyecto. No afirma que un
workflow, escáner o Quality Gate haya pasado por la sola presencia de su configuración.

| Requisito                         | Evidencia en el repositorio                                       |
| --------------------------------- | ----------------------------------------------------------------- |
| Go y dependencias exactas         | `apps/api/go.mod`, `go.sum`, `apps/web/spa-server/go.mod`         |
| Versiones frontend exactas        | `.nvmrc`, `package.json`, `pnpm-workspace.yaml`, `pnpm-lock.yaml` |
| API-first OpenAPI 3.1             | Structs Huma, `contracts/openapi.yaml`, `pnpm codegen`            |
| Handler → Service → Store         | `apps/api/internal/{api,service,store}`                           |
| PostgreSQL exclusivo              | Compose, migraciones goose y queries sqlc                         |
| Idempotencia y concurrencia       | Transacciones, claves por actor y E2E con rechazos                |
| Auditoría atómica                 | Tabla `auditoria` y transacción de movimientos                    |
| JWT y permisos                    | `internal/auth`, middleware y matriz `acceso_test.go`             |
| Errores sin internos              | Respuestas Problem Details y mensajes controlados                 |
| Configuración validada            | `internal/config`, `.env.example`                                 |
| Sesión sin almacenamiento web     | Cookie de Identidad, clientes con credenciales                    |
| Microcomponentes autónomos        | `apps/web/src/componentes`, paquete `@pingeso-ejecu/stock-ui`     |
| Runtime sin root                  | API/web distroless, migraciones UID 65532                         |
| Bases fijadas por digest          | Tres Dockerfiles y Compose                                        |
| Migración antes de API            | Dependencias `service_completed_successfully` en Compose          |
| Hooks y detección temprana        | `.editorconfig`, `lefthook.yml`, verificación de límites          |
| Contrato sin cambios accidentales | `codegen`, diff limpio y oasdiff en CI                            |
| Controles de seguridad            | Gitleaks, govulncheck, audit, Hadolint, Trivy y Dockle en CI      |
| Calidad con cobertura real        | SonarQube y Quality Gate; E2E con binario instrumentado           |
| Trazabilidad de publicación       | SBOM, Cosign y promoción de imágenes verificadas                  |
| Actualizaciones controladas       | `.github/dependabot.yml`                                          |

## Digests completados en Hito 0

Se consultaron los índices multi-arquitectura con
`docker buildx imagetools inspect <imagen> --format '{{json .Manifest.Digest}}'`:

| Imagen exacta               | Digest del índice                                                         |
| --------------------------- | ------------------------------------------------------------------------- |
| `postgres:17.11`            | `sha256:ff87138949ad9a46f32ff23bd6852b52ce024a13dffe9b4087be98292cdd3b4f` |
| `alpine:3.23.7`             | `sha256:ff7f0c727dd796aa36787bfcd9a64bf35a6ce72a73451b3359fde849b6703d63` |
| `axllent/mailpit:v1.31.3`   | `sha256:ed9b00c609e77e99c79b93f1178255ebc271868920f2c69a8d166bd5634ed10d` |
| `golang:1.26.11-alpine3.23` | `sha256:23f198931d4bd83b4f5a613bd75656257ae8bf832dcbc2db80e1050b5219ca2a` |

Los digests de Go, Node y distroless se tomaron de DAS. Una actualización de versión o
digest debe realizarse en un PR propio y pasar todo el pipeline.

## CI y publicación

`calidad`, `seguridad` e `imagen` corren en PR y push a `main`. Las Actions están fijadas
a los SHA de DAS, con versiones indicadas en comentarios. El comparador de contrato usa
la base del PR o el commit previo del push. Si todavía no existe un contrato en esa base,
registra explícitamente la creación de la primera referencia; los cambios posteriores sí
se comparan.

Las tres imágenes se construyen, escanean con Trivy y verifican con Dockle. La opción
manual `publicar=true`, sólo en `main`, etiqueta y publica esas mismas imágenes dentro
del job que las verificó, sin reconstruirlas. Después firma las imágenes y adjunta SBOM,
contrato y notas de release. El paquete npm se publica con su versión declarada.

No hay exclusiones de CVE ni omisiones condicionales del Quality Gate. Los reportes Go y
frontend son obligatorios. La configuración requiere al menos 60% de cobertura de código
nuevo en el Quality Gate, además del resto de umbrales definidos por DAS; esos umbrales
se establecen en el proyecto SonarQube, no se simulan en un archivo local.

## Pendiente fuera del código

- Recibir los contratos oficiales de Productos e Identidad y el documento de componentes;
  confirmar número del servicio, puertos, claims y permisos.
- Organización confirmada: `PINGESO-EJECU`. El paquete npm es `@pingeso-ejecu/stock-ui` y las imágenes
  quedan en `ghcr.io/pingeso-ejecu/`. El módulo Go conserva `github.com/pingeso/stock`, que no se publica.
- Verificar el proyecto SonarCloud `LucianoXHDX_stock_pingeso` (org `lucianoxhdx`), definir código nuevo, configurar el Quality Gate DAS y confirmar `SONAR_TOKEN`/`SONAR_HOST_URL` como GitHub Secrets.
- Activar la protección de `main`: PR obligatorio, una aprobación y checks requeridos.
- Verificar permisos de publicación de GHCR/GitHub Packages y OIDC para Cosign.
- Ejecutar el pipeline completo en el repositorio conectado y adjuntar sus enlaces y
  resultados; no se dispone aquí de un proyecto SonarQube ni de credenciales externas.
- Probar y documentar el bloqueo del hook Gitleaks en un commit temporal con un secreto
  falso; nunca incorporar un secreto real para esta prueba.

La integración real requiere credenciales válidas y que los servicios dependientes sean
alcanzables. La demostración MSW y los proveedores controlados del E2E no sustituyen la
aceptación entre grupos.

La ejecución inicial de audit, govulncheck y Trivy encontró vulnerabilidades. Las
transitivas Go no prescritas se seleccionaron en versiones corregidas y Trivy filesystem
terminó sin HIGH/CRITICAL con fix. Se conservaron las versiones de la tabla: persisten
hallazgos del toolchain Go, Vite y Faker transitivo de Prism, y los bloqueos de CI siguen activos. Véase
[verificacion.md](verificacion.md) para los resultados iniciales y posteriores.
