import { createRoot } from 'react-dom/client'
import { App } from './app'
import './estilos.css'

async function iniciar() {
  const demo = new URLSearchParams(window.location.search).get('demo') === '1'
  if (demo) {
    const { worker } = await import('./mocks/browser')
    await worker.start({
      onUnhandledRequest: 'bypass',
      serviceWorker: { url: '/mockServiceWorker.js' },
    })
  } else if ('serviceWorker' in navigator) {
    const registros = await navigator.serviceWorker.getRegistrations()
    await Promise.all(
      registros
        .filter((registro) =>
          registro.active?.scriptURL.endsWith('/mockServiceWorker.js'),
        )
        .map((registro) => registro.unregister()),
    )
  }
  const contenedor = document.getElementById('root')
  if (!contenedor) throw new Error('No se encontró el contenedor de la aplicación')
  createRoot(contenedor).render(<App demo={demo} />)
}

void iniciar().catch((error: unknown) => {
  const contenedor = document.getElementById('root')
  if (contenedor)
    contenedor.textContent = `No se pudo iniciar la aplicación: ${error instanceof Error ? error.message : 'error desconocido'}. Recarga la página para reintentar.`
})
