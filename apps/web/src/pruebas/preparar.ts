import { afterAll, afterEach, beforeAll } from 'vitest'
import { setupServer } from 'msw/node'
import { handlers, reiniciarDemo } from '../mocks/handlers'

export const servidor = setupServer(...handlers)

Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true })
beforeAll(() => servidor.listen({ onUnhandledRequest: 'error' }))
afterEach(() => {
  servidor.resetHandlers()
  reiniciarDemo()
})
afterAll(() => servidor.close())
