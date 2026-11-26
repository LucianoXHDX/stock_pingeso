-- +goose Up
CREATE TABLE existencias (
    producto_id uuid PRIMARY KEY,
    cantidad bigint NOT NULL DEFAULT 0 CHECK (cantidad BETWEEN 0 AND 1000000000),
    version bigint NOT NULL DEFAULT 0 CHECK (version >= 0),
    actualizado_en timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE movimientos (
    id uuid PRIMARY KEY,
    producto_id uuid NOT NULL REFERENCES existencias(producto_id),
    tipo text NOT NULL CHECK (tipo IN ('entrada', 'salida', 'ajuste')),
    cantidad bigint NOT NULL CHECK (cantidad BETWEEN 1 AND 1000000000),
    saldo_anterior bigint NOT NULL CHECK (saldo_anterior BETWEEN 0 AND 1000000000),
    saldo_posterior bigint NOT NULL CHECK (saldo_posterior BETWEEN 0 AND 1000000000),
    motivo varchar(500) NOT NULL CHECK (char_length(motivo) BETWEEN 3 AND 500),
    referencia varchar(200) NOT NULL DEFAULT '',
    actor_id uuid NOT NULL,
    clave_idempotencia varchar(128) NOT NULL CHECK (char_length(clave_idempotencia) BETWEEN 8 AND 128),
    huella char(64) NOT NULL CHECK (huella ~ '^[a-f0-9]{64}$'),
    creado_en timestamptz NOT NULL,
    UNIQUE (actor_id, clave_idempotencia),
    CONSTRAINT movimiento_coherente CHECK (
        (tipo = 'entrada' AND saldo_posterior = saldo_anterior + cantidad)
        OR (tipo = 'salida' AND saldo_posterior = saldo_anterior - cantidad)
        OR (tipo = 'ajuste' AND abs(saldo_posterior - saldo_anterior) = cantidad)
    )
);

CREATE INDEX movimientos_producto_fecha ON movimientos (producto_id, creado_en DESC, id DESC);
CREATE INDEX movimientos_fecha ON movimientos (creado_en DESC, id DESC);

CREATE TABLE auditoria (
    movimiento_id uuid PRIMARY KEY REFERENCES movimientos(id),
    actor_id uuid NOT NULL,
    accion text NOT NULL,
    datos jsonb NOT NULL,
    creado_en timestamptz NOT NULL
);

-- +goose Down
DROP TABLE auditoria;
DROP TABLE movimientos;
DROP TABLE existencias;
