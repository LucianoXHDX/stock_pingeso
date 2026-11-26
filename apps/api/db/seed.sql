-- Identificador reservado para la demostración local. El proveedor Productos debe
-- configurarlo por separado para operar; este script no escribe datos ajenos.
INSERT INTO existencias (producto_id, cantidad, version)
VALUES ('10000000-0000-4000-8000-000000000001', 0, 0)
ON CONFLICT (producto_id) DO NOTHING;
