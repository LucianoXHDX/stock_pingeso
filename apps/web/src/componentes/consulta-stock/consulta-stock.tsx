import { useId } from 'react'
import { fechaLegible, type PropiedadesStock } from '../compartido/tipos'
import { useConsultaStock } from './use-consulta-stock'

export function ConsultaStock({ apiBaseUrl, permisos }: PropiedadesStock) {
  const autorizado = permisos.includes('stock:consultar')
  const estado = useConsultaStock(apiBaseUrl, autorizado)
  const id = useId()

  return (
    <section className="stock-ui stock-card" aria-labelledby={`${id}-titulo`}>
      <header className="stock-card-header">
        <div>
          <p className="stock-eyebrow">Inventario disponible</p>
          <h2 id={`${id}-titulo`}>Existencias</h2>
        </div>
        <button
          className="stock-button stock-button-secondary"
          type="button"
          onClick={() => void estado.cargar()}
          disabled={estado.cargando || !autorizado}
        >
          Actualizar <span aria-hidden="true">↻</span>
        </button>
      </header>
      {!autorizado ? (
        <output className="stock-notice">
          Necesitas el permiso stock:consultar para ver las existencias.
        </output>
      ) : (
        <>
          <form
            className="stock-search"
            onSubmit={(evento) => {
              evento.preventDefault()
              estado.buscar()
            }}
          >
            <div>
              <label htmlFor={`${id}-producto`}>
                Buscar por identificador de producto
              </label>
              <input
                id={`${id}-producto`}
                value={estado.producto}
                onChange={(evento) => estado.establecerProducto(evento.target.value)}
                placeholder="UUID del producto · vacío para ver todos"
                pattern="[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}"
              />
            </div>
            <button
              className="stock-button stock-button-primary"
              disabled={estado.cargando}
              type="submit"
            >
              Buscar
            </button>
          </form>
          {estado.error && (
            <div className="stock-error" role="alert">
              <p>{estado.error}</p>
              <button type="button" onClick={() => void estado.cargar()}>
                Reintentar consulta
              </button>
            </div>
          )}
          {estado.cargando && (
            <output className="stock-loading">Consultando existencias…</output>
          )}
          {!estado.cargando && !estado.error && (
            <>
              <div className="stock-table-scroll">
                <table>
                  <caption className="stock-sr-only">
                    Cantidades actuales por producto
                  </caption>
                  <thead>
                    <tr>
                      <th scope="col">Producto</th>
                      <th scope="col" className="stock-number">
                        Disponible
                      </th>
                      <th scope="col">Versión</th>
                      <th scope="col">Último cambio</th>
                    </tr>
                  </thead>
                  <tbody>
                    {estado.existencias.map((existencia) => (
                      <tr key={existencia.producto_id}>
                        <td>
                          <span className="stock-product-icon" aria-hidden="true">
                            ▧
                          </span>
                          <code title={existencia.producto_id}>
                            {existencia.producto_id}
                          </code>
                        </td>
                        <td className="stock-number">
                          <span
                            className={`stock-quantity ${existencia.cantidad === 0 ? 'stock-empty' : ''}`}
                          >
                            {existencia.cantidad.toLocaleString('es-CL')}{' '}
                            <small>un.</small>
                          </span>
                        </td>
                        <td>
                          <span className="stock-version">v{existencia.version}</span>
                        </td>
                        <td className="stock-muted">
                          {fechaLegible(existencia.actualizado_en)}
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
              {estado.existencias.length === 0 && (
                <div className="stock-empty-state">
                  <span aria-hidden="true">▧</span>
                  <h3>Aún no hay existencias</h3>
                  <p>Registra una entrada para comenzar el inventario de un producto.</p>
                </div>
              )}
              <footer className="stock-pagination">
                <span>
                  {estado.total} {estado.total === 1 ? 'producto' : 'productos'}
                  {estado.consulta ? ' encontrado' : ' en inventario'}
                </span>
                {!estado.consulta && (
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
                )}
              </footer>
            </>
          )}
        </>
      )}
    </section>
  )
}
