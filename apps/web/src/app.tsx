import { useState } from 'react'
import { ConsultaStock } from './componentes/consulta-stock/consulta-stock'
import { MovimientoStock } from './componentes/movimiento-stock/movimiento-stock'
import { HistorialStock } from './componentes/historial-stock/historial-stock'

const permisos = ['stock:consultar', 'stock:operar', 'stock:administrar']

export function App({ demo }: { demo: boolean }) {
  const [vista, establecerVista] = useState<'inventario' | 'historial'>('inventario')
  return (
    <div className="stock-demo">
      <a className="stock-skip-link" href="#contenido">
        Ir al contenido
      </a>
      <aside className="stock-sidebar" aria-label="Navegación principal">
        <a href="/" className="stock-brand" aria-label="Stock inicio">
          <span aria-hidden="true">▧</span>
          <span>
            stock<span className="stock-brand-dot">.</span>
          </span>
        </a>
        <div className="stock-workspace">
          <span className="stock-workspace-icon" aria-hidden="true">
            08
          </span>
          <div>
            <strong>Grupo 8</strong>
            <span>Gestión de inventario</span>
          </div>
          <span aria-hidden="true">⌄</span>
        </div>
        <p className="stock-nav-label">ESPACIO DE TRABAJO</p>
        <nav>
          <button
            className={vista === 'inventario' ? 'stock-nav-active' : ''}
            aria-current={vista === 'inventario' ? 'page' : undefined}
            onClick={() => establecerVista('inventario')}
          >
            <span aria-hidden="true">▦</span>Inventario
            <span className="stock-nav-arrow" aria-hidden="true">
              ↗
            </span>
          </button>
          <button
            className={vista === 'historial' ? 'stock-nav-active' : ''}
            aria-current={vista === 'historial' ? 'page' : undefined}
            onClick={() => establecerVista('historial')}
          >
            <span aria-hidden="true">◷</span>Movimientos
            <span className="stock-nav-arrow" aria-hidden="true">
              ↗
            </span>
          </button>
        </nav>
        <div className="stock-sidebar-bottom">
          <div className="stock-module-note">
            <span aria-hidden="true">◇</span>
            <strong>Una base común.</strong>
            <p>El inventario que conecta a todos los proyectos.</p>
          </div>
          <div className="stock-sidebar-footer">
            <span className="stock-avatar">G8</span>
            <div>
              <strong>Módulo STOCK</strong>
              <span>Contrato v1.0.0</span>
            </div>
            <span aria-hidden="true">⋯</span>
          </div>
        </div>
      </aside>
      <div className="stock-main-column">
        <header className="stock-topbar">
          <div>
            Espacio de trabajo <span aria-hidden="true">/</span>{' '}
            <strong>{vista === 'inventario' ? 'Inventario' : 'Movimientos'}</strong>
          </div>
          <div className="stock-topbar-status">
            <span className={`stock-status-dot ${demo ? 'stock-demo-dot' : ''}`} />
            {demo ? 'Demostración local' : 'Conexión con la API'}
            <span className="stock-topbar-divider" />
            <span className="stock-avatar stock-avatar-small">G8</span>
          </div>
        </header>
        <main id="contenido" className="stock-main">
          <div className="stock-page-heading">
            <div>
              <p className="stock-page-eyebrow">
                <span /> CONTROL DE STOCK
              </p>
              <h1>
                {vista === 'inventario'
                  ? 'Todo tu inventario, al día.'
                  : 'Cada movimiento cuenta.'}
              </h1>
              <p>
                {vista === 'inventario'
                  ? 'Consulta existencias y registra los cambios de tu inventario en un solo lugar.'
                  : 'Consulta la trazabilidad de entradas, salidas y ajustes de tus productos.'}
              </p>
            </div>
            <span className="stock-heading-label">
              GRUPO 08 <span aria-hidden="true">↗</span>
            </span>
          </div>
          <div
            className={`stock-environment ${demo ? 'stock-environment-demo' : ''}`}
            role="note"
          >
            <span aria-hidden="true">{demo ? '◉' : '◇'}</span>
            <div>
              <strong>
                {demo ? 'Modo demostración' : 'Aplicación local de integración'}
              </strong>
              <span>
                {demo
                  ? 'Datos simulados en memoria. Los cambios se reinician al recargar.'
                  : 'Utiliza la sesión de tu aplicación de identidad. El backend valida cada permiso.'}
              </span>
            </div>
            {demo ? (
              <a href="/">Usar API real ↗</a>
            ) : (
              <a href="/?demo=1">Ver demostración ↗</a>
            )}
          </div>
          {demo && (
            <p className="stock-demo-product">
              Producto de ejemplo: <code>11111111-1111-4111-8111-111111111111</code>
            </p>
          )}
          {vista === 'inventario' ? (
            <div className="stock-dashboard-grid">
              <ConsultaStock apiBaseUrl="" permisos={permisos} />
              <MovimientoStock apiBaseUrl="" permisos={permisos} />
            </div>
          ) : (
            <HistorialStock apiBaseUrl="" permisos={permisos} />
          )}
          <footer className="stock-page-footer">
            <span>
              STOCK <span aria-hidden="true">/</span> Grupo 8
            </span>
            <span>Datos conectados. Decisiones claras.</span>
          </footer>
        </main>
      </div>
    </div>
  )
}
