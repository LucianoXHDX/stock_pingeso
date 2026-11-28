# Mocks locales de Identidad y Productos

La API exige al arrancar el JWKS de Identidad (grupo 6) y la URL de Productos (grupo 3).
Mientras esos servicios no estén disponibles, `docker-compose.mocks.yaml` agrega un
servicio `mocks` que los simula. Permite levantar PostgreSQL, migraciones, API y web en
un equipo local sin depender de otros grupos.

Es sólo para desarrollo. No se usa en CI ni se publica. La demostración del front con
`?demo=1` (MSW) sigue funcionando sin Docker.

## Qué simula

| Servicio  | Ruta del mock                         | Comportamiento                                                                       |
| --------- | ------------------------------------- | ------------------------------------------------------------------------------------ |
| Identidad | `GET /.well-known/jwks.json`          | Llave pública Ed25519 generada en cada arranque                                      |
| Identidad | `GET /dev/sesion?perfil=<perfil>`     | Crea la cookie `sesion` (HttpOnly, Secure, SameSite=Lax) y redirige al front         |
| Identidad | `GET /dev/token?perfil=<perfil>`      | Devuelve un JWT para `Authorization: Bearer`                                         |
| Identidad | `GET /dev/salir`                      | Borra la cookie                                                                      |
| Productos | `GET /api/v1/productos/{producto_id}` | Contrato propuesto en `contracts/externos/productos.yaml`: 200, 404 o 401 sin Bearer |
| Productos | `GET /api/v1/productos`               | Lista los productos del mock (ayuda, no es parte del contrato)                       |

Perfiles: `admin` (consultar, operar, administrar), `operacion` (consultar, operar) y
`consulta` (consultar). Emisor `http://identidad-mock`, audiencia `stock`. Las sesiones
duran 8 horas y dejan de valer al reiniciar el contenedor `mocks`.

Productos existentes por defecto: `10000000-0000-4000-8000-000000000001` (el del seed)
hasta `10000000-0000-4000-8000-000000000005`. Cualquier otro UUID responde 404. Se pueden
reemplazar con `MOCK_PRODUCTOS="uuid=Nombre,uuid=Nombre"` en el servicio `mocks`.

## Uso

1. Docker Desktop abierto. Copiar `.env.example` a `.env` y completar `POSTGRES_USER`,
   `POSTGRES_PASSWORD` y los mismos valores dentro de `DATABASE_URL`. Las variables
   `AUTH_*` y `PRODUCTOS_URL` pueden quedar como están: el override las reemplaza.
2. Levantar todo:

   ```sh
   pnpm docker:mocks
   pnpm docker:mocks:estado
   ```

3. Abrir `http://localhost:5129`, elegir un perfil y volver a `http://localhost:5120`.
   Usar `localhost`, no `127.0.0.1`: la cookie y el control de `Origin` dependen del nombre.
4. Detener con `pnpm docker:mocks:down`. Los datos quedan en el volumen de PostgreSQL.

Con curl:

```sh
TOKEN=$(curl -s "http://localhost:5129/dev/token?perfil=admin")
curl -H "Authorization: Bearer $TOKEN" http://127.0.0.1:5121/api/v1/existencias
```

Cuando Identidad y Productos publiquen sus servicios, basta con usar el compose base y
completar `AUTH_*` y `PRODUCTOS_URL` en `.env`.

## Simular Stock para otros grupos (Prism)

Los grupos que consumen Stock (Catálogo, Registro de venta, Boleta e Historial) pueden
probar contra el contrato sin levantar PostgreSQL ni los mocks de arriba:

```sh
pnpm mock:stock
```

Levanta Prism **5.16.0** en `http://127.0.0.1:5122` con `contracts/openapi.yaml`. Responde
los ejemplos del contrato y valida las solicitudes como la API real: sin
`Authorization: Bearer <cualquier valor>` devuelve 401, y un `POST /api/v1/movimientos` sin
`Idempotency-Key` devuelve 422. No guarda estado: cada respuesta sale del contrato.

```sh
curl -H "Authorization: Bearer prueba" http://127.0.0.1:5122/api/v1/existencias
```

Prism se ejecuta con `npx` en su versión fija y no desde `node_modules`. La dependencia
`@stoplight/prism-cli` de `apps/web` no arranca porque el override de `@faker-js/faker`
10.5.0 (necesario para que `pnpm audit` pase) rompe a `postman-collection`, que usa la API
antigua de Faker. Ejecutado aparte, Prism usa sus propias dependencias y el audit del
proyecto no cambia.

Otro grupo también puede usarlo con su copia del contrato:

```sh
npx --yes @stoplight/prism-cli@5.16.0 mock contracts/externos/stock.yaml --port 5122
```
