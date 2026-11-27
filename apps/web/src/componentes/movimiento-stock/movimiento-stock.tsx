import { useId } from 'react'
import {
  nombreTipo,
  type PropiedadesStock,
  type TipoMovimiento,
} from '../compartido/tipos'
import { useMovimientoStock } from './use-movimiento-stock'

export function MovimientoStock({ apiBaseUrl, permisos }: PropiedadesStock) {
  const id = useId()
  const estado = useMovimientoStock(apiBaseUrl)
  const puedeOperar = permisos.includes('stock:operar')
  const puedeAjustar = permisos.includes('stock:administrar')
  const autorizado = estado.tipo === 'ajuste' ? puedeAjustar : puedeOperar
  const tipos: TipoMovimiento[] = ['entrada', 'salida', 'ajuste']

  return (
    <section className="stock-ui stock-card" aria-labelledby={`${id}-titulo`}>
      <header className="stock-card-header">
        <div>
          <p className="stock-eyebrow">Control de inventario</p>
          <h2 id={`${id}-titulo`}>Registrar movimiento</h2>
        </div>
        <span className="stock-section-symbol" aria-hidden="true">
          ↗
        </span>
      </header>
      {!puedeOperar && !puedeAjustar ? (
        <output className="stock-notice">
          Necesitas permiso de operación o administración para registrar movimientos.
        </output>
      ) : (
        <form
          className="stock-movement-form"
          onSubmit={(evento) => {
            evento.preventDefault()
            if (autorizado) void estado.registrar()
          }}
        >
          <fieldset disabled={estado.enviando}>
            <legend>Tipo de movimiento</legend>
            <div className="stock-type-selector">
              {tipos.map((tipo) => (
                <label
                  key={tipo}
                  className={estado.tipo === tipo ? 'stock-type-active' : ''}
                >
                  <input
                    type="radio"
                    name={`${id}-tipo`}
                    value={tipo}
                    checked={estado.tipo === tipo}
                    disabled={tipo === 'ajuste' ? !puedeAjustar : !puedeOperar}
                    onChange={() => estado.establecerTipo(tipo)}
                  />
                  <span aria-hidden="true">
                    {tipo === 'entrada' ? '↙' : tipo === 'salida' ? '↗' : '⇄'}
                  </span>
                  {nombreTipo(tipo)}
                </label>
              ))}
            </div>
          </fieldset>
          <p className="stock-help">
            {estado.tipo === 'ajuste'
              ? 'Establece el saldo final del producto. Requiere su versión actual para proteger cambios recientes.'
              : estado.tipo === 'salida'
                ? 'Descuenta unidades del inventario. La cantidad disponible nunca puede quedar bajo cero.'
                : 'Suma unidades al inventario de un producto del catálogo.'}
          </p>
          <label htmlFor={`${id}-producto`}>
            Identificador de producto <span aria-hidden="true">*</span>
            <input
              id={`${id}-producto`}
              value={estado.producto}
              onChange={(evento) => estado.establecerProducto(evento.target.value)}
              placeholder="xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx"
              required
              pattern="[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}"
              disabled={estado.enviando}
            />
          </label>
          <div className="stock-form-row">
            <label htmlFor={`${id}-cantidad`}>
              {estado.tipo === 'ajuste' ? 'Saldo final' : 'Cantidad'}{' '}
              <span aria-hidden="true">*</span>
              <input
                id={`${id}-cantidad`}
                type="number"
                min={estado.tipo === 'ajuste' ? 0 : 1}
                max={1000000000}
                step={1}
                value={estado.cantidad}
                onChange={(evento) => estado.establecerCantidad(evento.target.value)}
                placeholder="0"
                required
                disabled={estado.enviando}
              />
            </label>
            {estado.tipo === 'ajuste' && (
              <label htmlFor={`${id}-version`}>
                Versión actual <span aria-hidden="true">*</span>
                <input
                  id={`${id}-version`}
                  type="number"
                  min={0}
                  step={1}
                  value={estado.version}
                  onChange={(evento) => estado.establecerVersion(evento.target.value)}
                  placeholder="Ej. 3"
                  required
                  disabled={estado.enviando}
                />
              </label>
            )}
          </div>
          <label htmlFor={`${id}-motivo`}>
            Motivo <span aria-hidden="true">*</span>
            <textarea
              id={`${id}-motivo`}
              value={estado.motivo}
              onChange={(evento) => estado.establecerMotivo(evento.target.value)}
              placeholder="Describe el motivo de este movimiento"
              minLength={3}
              maxLength={500}
              required
              rows={3}
              disabled={estado.enviando}
            />
          </label>
          <label htmlFor={`${id}-referencia`}>
            Referencia <span className="stock-optional">Opcional</span>
            <input
              id={`${id}-referencia`}
              value={estado.referencia}
              onChange={(evento) => estado.establecerReferencia(evento.target.value)}
              placeholder="Orden de compra, pedido o documento"
              maxLength={200}
              disabled={estado.enviando}
            />
          </label>
          {estado.error && (
            <p role="alert" className="stock-error">
              {estado.error} Puedes reintentar el mismo formulario de manera segura.
            </p>
          )}
          {estado.resultado && (
            <div aria-live="polite" className="stock-success">
              <strong>Movimiento registrado</strong>
              <p>
                {nombreTipo(estado.resultado.tipo)} confirmada. Saldo:{' '}
                {estado.resultado.saldo_anterior} → {estado.resultado.saldo_posterior}{' '}
                unidades.
              </p>
              <p>Actualiza existencias o historial para ver el cambio.</p>
            </div>
          )}
          <button
            className="stock-button stock-button-primary stock-submit"
            type="submit"
            disabled={estado.enviando || !autorizado}
          >
            {estado.enviando ? 'Registrando movimiento…' : `Registrar ${estado.tipo}`}
            <span aria-hidden="true">→</span>
          </button>
          <p className="stock-footnote">
            Cada movimiento conserva su motivo y responsable.
          </p>
        </form>
      )}
    </section>
  )
}
