# Integración con otros grupos

La documentación suministrada incluye DAS y la división de grupos, pero no el documento
oficial de componentes ni los contratos de los proveedores. Estas decisiones permiten
avanzar y deben ratificarse con el jefe de proyecto antes del ensamblaje.

| Dato                | Decisión actual                          | Confirmación necesaria                           |
| ------------------- | ---------------------------------------- | ------------------------------------------------ |
| Responsable         | Grupo 8, STOCK                           | Coincide con la asignación recibida              |
| Catálogo            | Productos, grupo 3                       | API oficial y semántica de productos habilitados |
| Número del servicio | 2 según posición de STOCK en la imagen   | Documento de componentes                         |
| Puertos locales     | Web 5120, API 5121                       | Número oficial del servicio                      |
| Identidad           | JWT RS256/EdDSA mediante cookie o Bearer | Issuer, audience, JWKS y permisos oficiales      |
| Namespace           | `pingeso` para módulo Go y npm           | Organización de GitHub del curso                 |

## Productos

`contracts/externos/productos.yaml` es una propuesta de OpenAPI 3.1 que permite generar
el cliente Go y probar la dependencia. **No es un contrato oficial recibido.** La
propuesta mínima es:

```http
GET /api/v1/productos/{producto_id}
Authorization: Bearer <JWT del usuario>
X-Request-Id: <id de correlación>
```

Una respuesta 200 contiene `{ "id": "<UUID>" }`; 404 indica producto inexistente.
El cliente valida la identidad del producto devuelto. Los problemas de conexión,
timeout o respuestas no válidas se traducen a 503 y no confirman el movimiento local.
El timeout es de cinco segundos.

Cuando llegue el contrato publicado por Productos, se reemplaza la propuesta y se
regenera el cliente con `make clients`. No se debe adaptar el proveedor a este archivo
sin acordarlo. Los mocks del E2E implementan únicamente esta interfaz propuesta.

STOCK no crea productos ni consulta la base del catálogo. El seed de desarrollo sólo
registra una existencia vacía del UUID `10000000-0000-4000-8000-000000000001`; ese
producto debe existir por separado en el ambiente de Productos para operar con él.

## Identidad y sesión

Las variables obligatorias son `AUTH_JWKS_URL`, `AUTH_ISSUER` y `AUTH_AUDIENCE`.
`SESSION_COOKIE_NAME` vale `sesion` si se omite. Las claves públicas se cargan desde
JWKS; nunca se distribuye una clave privada a STOCK.

El JWT propuesto usa los claims registrados `sub` (UUID de usuario), `iss`, `aud`, `exp`
y, si corresponde, `iat`/`nbf`. Un arreglo `permisos` contiene permisos concretos:

```json
{
  "sub": "20000000-0000-4000-8000-000000000001",
  "iss": "https://identidad.ejemplo.invalid",
  "aud": "stock",
  "exp": 1900000000,
  "permisos": ["stock:consultar", "stock:operar"]
}
```

Este JSON describe el formato; no es una credencial. El servicio sólo acepta tokens
firmados por las llaves del JWKS, con algoritmo RS256 o EdDSA, emisor y audiencia
configurados y expiración válida. Los nombres finales de claims y permisos deben
acordarse con el proveedor. STOCK verifica permisos, no deduce privilegios del nombre
de un rol.

| Perfil propuesto | Consultar         | Entrada/salida | Ajustar             |
| ---------------- | ----------------- | -------------- | ------------------- |
| Consulta         | `stock:consultar` | No             | No                  |
| Operación        | `stock:consultar` | `stock:operar` | No                  |
| Administración   | `stock:consultar` | `stock:operar` | `stock:administrar` |

Los permisos son explícitos: `stock:administrar` por sí solo no implica los demás.
El servicio de Identidad es responsable de emitir la cookie con `HttpOnly`, `Secure` y
`SameSite=Lax`. STOCK no tiene login ni recibe contraseñas. El producto anfitrión compone
el login y la sesión bajo el dominio o gateway apropiado.

Las escrituras con cookie requieren `Origin` igual a `FRONTEND_URL`; las llamadas
entre servicios pueden reenviar el JWT como Bearer. El proxy de la app web reenvía
`/api` al backend sin exponer credenciales en variables públicas ni habilitar CORS global.

## Consumidores de STOCK

Copiar el contrato publicado de esta versión, generar el cliente y llamar por REST.
Mantener `Idempotency-Key` en todos los reintentos de una misma mutación y propagar
`X-Request-Id`. Ante un 409 por falta de stock, mostrar el rechazo al usuario. Ante un
409 de versión, volver a consultar antes de proponer otro ajuste. Ante un 503, reintentar
con la misma clave para evitar duplicados si el resultado anterior fue incierto.

Una cancelación de una operación ya confirmada se registra con un movimiento inverso,
otra clave y referencia al original. No existe un borrado administrativo del historial.

Los consumidores pueden montar los microcomponentes de `@pingeso/stock-ui` bajo su
propio router. Deben proporcionar una URL base accesible desde el navegador, permisos y
la sesión del usuario; los componentes no administran el login ni un estado global.
