-- name: ConsultarExistencia :one
SELECT producto_id::text AS producto_id, cantidad, version, actualizado_en
FROM existencias WHERE producto_id = sqlc.arg(producto_id)::text::uuid;

-- name: ContarExistencias :one
SELECT count(*) FROM existencias
WHERE sqlc.arg(producto_id)::text = '' OR producto_id::text = sqlc.arg(producto_id)::text;

-- name: ListarExistencias :many
SELECT producto_id::text AS producto_id, cantidad, version, actualizado_en
FROM existencias
WHERE sqlc.arg(producto_id)::text = '' OR producto_id::text = sqlc.arg(producto_id)::text
ORDER BY producto_id
LIMIT sqlc.arg(limite)::integer OFFSET sqlc.arg(desplazamiento)::integer;

-- name: ContarMovimientos :one
SELECT count(*) FROM movimientos
WHERE (sqlc.arg(producto_id)::text = '' OR producto_id::text = sqlc.arg(producto_id)::text)
  AND (sqlc.arg(tipo)::text = '' OR tipo = sqlc.arg(tipo)::text);

-- name: ListarMovimientos :many
SELECT id::text AS id, producto_id::text AS producto_id, tipo, cantidad, saldo_anterior,
       saldo_posterior, motivo, referencia, actor_id::text AS actor_id, creado_en
FROM movimientos
WHERE (sqlc.arg(producto_id)::text = '' OR producto_id::text = sqlc.arg(producto_id)::text)
  AND (sqlc.arg(tipo)::text = '' OR tipo = sqlc.arg(tipo)::text)
ORDER BY creado_en DESC, id DESC
LIMIT sqlc.arg(limite)::integer OFFSET sqlc.arg(desplazamiento)::integer;

-- name: BuscarMovimiento :one
SELECT id::text AS id, producto_id::text AS producto_id, tipo, cantidad, saldo_anterior,
       saldo_posterior, motivo, referencia, actor_id::text AS actor_id, creado_en, huella
FROM movimientos
WHERE actor_id = sqlc.arg(actor_id)::text::uuid AND clave_idempotencia = sqlc.arg(clave_idempotencia)::text;

-- name: BloquearIdempotencia :exec
SELECT pg_advisory_xact_lock(hashtextextended(sqlc.arg(recurso)::text, 0));

-- name: CrearExistencia :exec
INSERT INTO existencias (producto_id) VALUES (sqlc.arg(producto_id)::text::uuid)
ON CONFLICT (producto_id) DO NOTHING;

-- name: BloquearExistencia :one
SELECT producto_id::text AS producto_id, cantidad, version, actualizado_en
FROM existencias WHERE producto_id = sqlc.arg(producto_id)::text::uuid
FOR UPDATE;

-- name: ActualizarExistencia :exec
UPDATE existencias SET cantidad = sqlc.arg(cantidad), version = version + 1,
    actualizado_en = sqlc.arg(actualizado_en)
WHERE producto_id = sqlc.arg(producto_id)::text::uuid;

-- name: InsertarMovimiento :exec
INSERT INTO movimientos (
    id, producto_id, tipo, cantidad, saldo_anterior, saldo_posterior, motivo,
    referencia, actor_id, clave_idempotencia, huella, creado_en
) VALUES (
    sqlc.arg(id)::text::uuid, sqlc.arg(producto_id)::text::uuid, sqlc.arg(tipo),
    sqlc.arg(cantidad), sqlc.arg(saldo_anterior), sqlc.arg(saldo_posterior),
    sqlc.arg(motivo), sqlc.arg(referencia), sqlc.arg(actor_id)::text::uuid,
    sqlc.arg(clave_idempotencia), sqlc.arg(huella), sqlc.arg(creado_en)
);

-- name: InsertarAuditoria :exec
INSERT INTO auditoria (movimiento_id, actor_id, accion, datos, creado_en)
SELECT id, actor_id, tipo,
    jsonb_build_object('producto_id', producto_id, 'cantidad', cantidad,
        'saldo_anterior', saldo_anterior, 'saldo_posterior', saldo_posterior,
        'motivo', motivo, 'referencia', referencia), creado_en
FROM movimientos WHERE id = sqlc.arg(movimiento_id)::text::uuid;
