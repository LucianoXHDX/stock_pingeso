import { useId } from 'react'
import {
  fechaLegible,
  nombreTipo,
  type PropiedadesStock,
  type TipoMovimiento,
} from '../compartido/tipos'
import { useHistorialStock } from './use-historial-stock'

export function HistorialStock({ apiBaseUrl, permisos }: PropiedadesStock) {
  const id = useId()
  const autorizado = permisos.includes('stock:consultar')
  const estado = useHistorialStock(apiBaseUrl, autorizado)

  return (
    <section className="stock-ui stock-card" aria-labelledby={`${id}-titulo`}>
      <header className="stock-card-header">
        <div>
          <p className="stock-eyebrow">Trazabilidad</p>
          <h2 id={`${id}-titulo`}>Historial de movimientos</h2>
        </div>
        <button
          className="stock-button stock-button-secondary"
          type="button"
          disabled={estado.cargando || !autorizado}
          onClick={() => void estado.cargar()}
        >
          Actualizar <span aria-hidden="true">↻</span>
        </button>
      </header>
      {!autorizado ? (
        <output className="stock-notice">
          Necesitas el permiso stock:consultar para ver el historial.
        </output>
      ) : (
        <>
          <form
            className="stock-history-filters"
            onSubmit={(evento) => {
              evento.preventDefault()
              estado.filtrar()
            }}
          >
            <label htmlFor={`${id}-producto`}>
              Producto
              <input
                id={`${id}-producto`}
                value={estado.producto}
                onChange={(evento) => estado.establecerProducto(evento.target.value)}
                placeholder="UUID del producto"
                pattern="[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}"
              />
            </label>
            <label htmlFor={`${id}-tipo`}>
              Tipo
              <select
                id={`${id}-tipo`}
                value={estado.tipo}
                onChange={(evento) =>
                  estado.establecerTipo(evento.target.value as TipoMovimiento | '')
                }
              >
                <option value="">Todos los movimientos</option>
                <option value="entrada">Entrada</option>
                <option value="salida">Salida</option>
                <option value="ajuste">Ajuste</option>
              </select>
            </label>
            <button
              className="stock-button stock-button-primary"
              type="submit"
              disabled={estado.cargando}
            >
              Aplicar filtros
            </button>
          </form>
          {estado.error && (
            <div role="alert" className="stock-error">
              <p>{estado.error}</p>
              <button type="button" onClick={() => void estado.cargar()}>
                Reintentar historial
              </button>
            </div>
          )}
          {estado.cargando && (
            <output className="stock-loading">Consultando movimientos…</output>
          )}
          {!estado.cargando && !estado.error && (
            <>
              <div className="stock-table-scroll">
                <table>
                  <caption className="stock-sr-only">
                    Movimientos y sus saldos confirmados
                  </caption>
                  <thead>
                    <tr>
                      <th scope="col">Movimiento / producto</th>
                      <th scope="col">Tipo</th>
                      <th scope="col" className="stock-number">
                        Cantidad
                      </th>
                      <th scope="col" className="stock-number">
                        Saldo
                      </th>
                      <th scope="col">Motivo y responsable</th>
                      <th scope="col">Fecha</th>
                    </tr>
                  </thead>
                  <tbody>
                    {estado.movimientos.map((movimiento) => (
                      <tr key={movimiento.id}>
                        <td>
                          <code title={movimiento.producto_id}>
                            {movimiento.producto_id}
                          </code>
                          <span className="stock-row-detail" title={movimiento.id}>
                            Mov. {movimiento.id.slice(0, 8)}
                          </span>
                        </td>
                        <td>
                          <span className={`stock-badge stock-badge-${movimiento.tipo}`}>
                            {nombreTipo(movimiento.tipo)}
                          </span>
                        </td>
                        <td className="stock-number stock-semibold">
                          {movimiento.tipo === 'entrada'
                            ? '+'
                            : movimiento.tipo === 'salida'
                              ? '−'
                              : ''}
                          {movimiento.cantidad.toLocaleString('es-CL')}
                        </td>
                        <td className="stock-number">
                          <span className="stock-muted">{movimiento.saldo_anterior}</span>{' '}
                          → {movimiento.saldo_posterior}
                        </td>
                        <td>
                          <span>{movimiento.motivo}</span>
                          <span className="stock-row-detail">
                            {movimiento.referencia ? `${movimiento.referencia} · ` : ''}
                            {movimiento.actor_id}
                          </span>
                        </td>
                        <td className="stock-muted">
                          {fechaLegible(movimiento.creado_en)}
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
              {estado.movimientos.length === 0 && (
                <div className="stock-empty-state">
                  <span aria-hidden="true">↗</span>
                  <h3>Sin movimientos para mostrar</h3>
                  <p>
                    Los movimientos confirmados aparecerán aquí. Revisa también tus
                    filtros.
                  </p>
                </div>
              )}
              <footer className="stock-pagination">
                <span>{estado.total} movimientos registrados</span>
                <div>
                  <button
                    type="button"
                    disabled={estado.desplazamiento === 0}
                    onClick={() =>
                      estado.establecerDesplazamiento(
                        Math.max(0, estado.desplazamiento - 10),
                      )
                    }
                  >
                    Anterior
                  </button>
                  <span>Página {Math.floor(estado.desplazamiento / 10) + 1}</span>
                  <button
                    type="button"
                    disabled={estado.desplazamiento + 10 >= estado.total}
                    onClick={() =>
                      estado.establecerDesplazamiento(estado.desplazamiento + 10)
                    }
                  >
                    Siguiente
                  </button>
                </div>
              </footer>
            </>
          )}
        </>
      )}
    </section>
  )
}
