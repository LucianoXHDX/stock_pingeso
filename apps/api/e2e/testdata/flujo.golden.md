# Resultado E2E STOCK

- Salud: healthz=200; readyz=200.
- Estado inicial: 0 existencias y 0 movimientos; producto conocido con saldo=0.
- Última unidad: 12 salidas simultáneas; aceptadas=1, conflictos=11, saldo=0, versión=2.
- Idempotencia concurrente: 8 respuestas 201; movimientos únicos=1, saldo=7, versión=1.
- Replay con Productos caído: mismo movimiento; consultas proveedor=0; conflicto de clave=409.
- Ajustes: objetivos 5 y 0; versión obsoleta=409; versión ausente=422; sin variación=422; saldo=0, versión=4.
- Límite de cantidad: saldo máximo=1000000000; desborde=422; versión=1.
- Contrato: 8 solicitudes inválidas rechazadas con 422 y errores RFC 9457 sin detalles internos.
- Productos: ausente=404; caído=503; JSON inválido=503; timeout=503; existencias nuevas=0.
- Consultas: ajustes filtrados=2; página de ajustes=1; página vacía=[] con total=6; existencias=3.
- Atomicidad: movimientos=6; auditorías=6; inconsistencias=0.
- Correlación: JWT y X-Request-Id propagados; fechas RFC 3339 en UTC.
- PostgreSQL no disponible: readyz=503; healthz=200; recuperación readyz=200.
