// Package store persiste STOCK mediante las consultas generadas por sqlc.
package store

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/pingeso/stock/apps/api/internal/repository"
	"github.com/pingeso/stock/apps/api/internal/service"
)

type Store struct {
	pool      *pgxpool.Pool
	consultas *repository.Queries
}

var _ service.Almacen = (*Store)(nil)

func Nuevo(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool, consultas: repository.New(pool)}
}

func (s *Store) Consultar(ctx context.Context, productoID string) (service.Existencia, error) {
	fila, err := s.consultas.ConsultarExistencia(ctx, productoID)
	if err != nil {
		return service.Existencia{}, traducirError(err)
	}
	return service.Existencia{
		ProductoID: fila.ProductoID, Cantidad: fila.Cantidad, Version: fila.Version,
		ActualizadoEn: fila.ActualizadoEn.UTC(),
	}, nil
}

func (s *Store) Listar(ctx context.Context, filtro service.Filtro) (service.ResultadoExistencias, error) {
	resultado := service.ResultadoExistencias{Datos: []service.Existencia{}}
	err := s.leer(ctx, func(consultas *repository.Queries) error {
		total, err := consultas.ContarExistencias(ctx, filtro.ProductoID)
		if err != nil {
			return err
		}
		filas, err := consultas.ListarExistencias(ctx, repository.ListarExistenciasParams{
			ProductoID: filtro.ProductoID, Limite: int32(filtro.Limite), Desplazamiento: int32(filtro.Desplazamiento),
		})
		if err != nil {
			return err
		}
		resultado.Total = total
		for _, fila := range filas {
			resultado.Datos = append(resultado.Datos, service.Existencia{
				ProductoID: fila.ProductoID, Cantidad: fila.Cantidad, Version: fila.Version,
				ActualizadoEn: fila.ActualizadoEn.UTC(),
			})
		}
		return nil
	})
	return resultado, err
}

func (s *Store) Historial(ctx context.Context, filtro service.Filtro) (service.ResultadoMovimientos, error) {
	resultado := service.ResultadoMovimientos{Datos: []service.Movimiento{}}
	err := s.leer(ctx, func(consultas *repository.Queries) error {
		total, err := consultas.ContarMovimientos(ctx, repository.ContarMovimientosParams{
			ProductoID: filtro.ProductoID, Tipo: filtro.Tipo,
		})
		if err != nil {
			return err
		}
		filas, err := consultas.ListarMovimientos(ctx, repository.ListarMovimientosParams{
			ProductoID: filtro.ProductoID, Tipo: filtro.Tipo,
			Limite: int32(filtro.Limite), Desplazamiento: int32(filtro.Desplazamiento),
		})
		if err != nil {
			return err
		}
		resultado.Total = total
		for _, fila := range filas {
			resultado.Datos = append(resultado.Datos, movimientoDesdeFila(fila))
		}
		return nil
	})
	return resultado, err
}

// Ambas consultas de una página ven la misma instantánea, incluso si se
// registran movimientos concurrentes entre el conteo y la lectura de filas.
func (s *Store) leer(ctx context.Context, operacion func(*repository.Queries) error) error {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return err
	}
	defer revertir(tx)
	if err := operacion(s.consultas.WithTx(tx)); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *Store) BuscarMovimiento(ctx context.Context, actorID, clave string) (service.Movimiento, string, error) {
	return buscarMovimiento(ctx, s.consultas, actorID, clave)
}

func buscarMovimiento(ctx context.Context, consultas *repository.Queries, actorID, clave string) (service.Movimiento, string, error) {
	fila, err := consultas.BuscarMovimiento(ctx, repository.BuscarMovimientoParams{
		ActorID: actorID, ClaveIdempotencia: clave,
	})
	if err != nil {
		return service.Movimiento{}, "", traducirError(err)
	}
	movimiento := service.Movimiento{
		ID: fila.ID, ProductoID: fila.ProductoID, Tipo: fila.Tipo, Cantidad: fila.Cantidad,
		SaldoAnterior: fila.SaldoAnterior, SaldoPosterior: fila.SaldoPosterior,
		Motivo: fila.Motivo, Referencia: fila.Referencia, ActorID: fila.ActorID,
		CreadoEn: fila.CreadoEn.UTC(),
	}
	return movimiento, fila.Huella, nil
}

func (s *Store) Ejecutar(ctx context.Context, actorID, clave, huella, productoID string, operacion func(service.Existencia) (service.Movimiento, error)) (service.Movimiento, error) {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return service.Movimiento{}, err
	}
	defer revertir(tx)
	consultas := s.consultas.WithTx(tx)
	// El bloqueo de clave precede al de existencia. También protege dos
	// solicitudes que reutilizan la misma clave para productos diferentes.
	if err := consultas.BloquearIdempotencia(ctx, "stock:"+actorID+":"+clave); err != nil {
		return service.Movimiento{}, err
	}
	anterior, huellaAnterior, err := buscarMovimiento(ctx, consultas, actorID, clave)
	if err == nil {
		if huellaAnterior != huella {
			return service.Movimiento{}, service.ErrConflicto
		}
		return anterior, nil
	}
	if !errors.Is(err, service.ErrNoEncontrado) {
		return service.Movimiento{}, err
	}
	if err := consultas.CrearExistencia(ctx, productoID); err != nil {
		return service.Movimiento{}, err
	}
	fila, err := consultas.BloquearExistencia(ctx, productoID)
	if err != nil {
		return service.Movimiento{}, err
	}
	movimiento, err := operacion(service.Existencia{
		ProductoID: fila.ProductoID, Cantidad: fila.Cantidad, Version: fila.Version,
		ActualizadoEn: fila.ActualizadoEn.UTC(),
	})
	if err != nil {
		return service.Movimiento{}, err
	}
	if err := consultas.ActualizarExistencia(ctx, repository.ActualizarExistenciaParams{
		Cantidad: movimiento.SaldoPosterior, ActualizadoEn: movimiento.CreadoEn, ProductoID: productoID,
	}); err != nil {
		return service.Movimiento{}, err
	}
	if err := consultas.InsertarMovimiento(ctx, repository.InsertarMovimientoParams{
		ID: movimiento.ID, ProductoID: productoID, Tipo: movimiento.Tipo, Cantidad: movimiento.Cantidad,
		SaldoAnterior: movimiento.SaldoAnterior, SaldoPosterior: movimiento.SaldoPosterior,
		Motivo: movimiento.Motivo, Referencia: movimiento.Referencia, ActorID: actorID,
		ClaveIdempotencia: clave, Huella: huella, CreadoEn: movimiento.CreadoEn,
	}); err != nil {
		return service.Movimiento{}, err
	}
	if err := consultas.InsertarAuditoria(ctx, movimiento.ID); err != nil {
		return service.Movimiento{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return service.Movimiento{}, err
	}
	return movimiento, nil
}

func movimientoDesdeFila(fila repository.ListarMovimientosRow) service.Movimiento {
	return service.Movimiento{
		ID: fila.ID, ProductoID: fila.ProductoID, Tipo: fila.Tipo, Cantidad: fila.Cantidad,
		SaldoAnterior: fila.SaldoAnterior, SaldoPosterior: fila.SaldoPosterior,
		Motivo: fila.Motivo, Referencia: fila.Referencia, ActorID: fila.ActorID,
		CreadoEn: fila.CreadoEn.UTC(),
	}
}

func traducirError(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return service.ErrNoEncontrado
	}
	return err
}

func revertir(tx pgx.Tx) {
	ctx, cancelar := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancelar()
	_ = tx.Rollback(ctx)
}
