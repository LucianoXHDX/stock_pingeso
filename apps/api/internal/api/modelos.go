package api

import "time"

const patronUUID = `^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`

type Existencia struct {
	ProductoID    string    `json:"producto_id" format:"uuid" pattern:"^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$" example:"11111111-1111-4111-8111-111111111111"`
	Cantidad      int64     `json:"cantidad" minimum:"0" maximum:"1000000000" example:"20"`
	Version       int64     `json:"version" minimum:"0" example:"1"`
	ActualizadoEn time.Time `json:"actualizado_en" doc:"Instante UTC del último movimiento. Valor cero si aún no hay movimientos."`
}

type Movimiento struct {
	ID             string    `json:"id" format:"uuid"`
	ProductoID     string    `json:"producto_id" format:"uuid"`
	Tipo           string    `json:"tipo" enum:"entrada,salida,ajuste" example:"entrada"`
	Cantidad       int64     `json:"cantidad" minimum:"1" maximum:"1000000000" doc:"Magnitud absoluta de la variación de stock; en ajustes el saldo objetivo está en saldo_posterior."`
	SaldoAnterior  int64     `json:"saldo_anterior" minimum:"0"`
	SaldoPosterior int64     `json:"saldo_posterior" minimum:"0"`
	Motivo         string    `json:"motivo" minLength:"3" maxLength:"500" example:"Reposición de mercadería"`
	Referencia     string    `json:"referencia" maxLength:"200" example:"OC-2026-001"`
	ActorID        string    `json:"actor_id" format:"uuid"`
	CreadoEn       time.Time `json:"creado_en"`
}

type EntradaConsulta struct {
	ProductoID string `path:"producto_id" format:"uuid" pattern:"^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$" example:"11111111-1111-4111-8111-111111111111"`
}
type EntradaListado struct {
	Limite         int `query:"limite" default:"20" minimum:"1" maximum:"100"`
	Desplazamiento int `query:"desplazamiento" default:"0" minimum:"0" maximum:"1000000"`
}
type EntradaHistorial struct {
	EntradaListado
	ProductoID string `query:"producto_id" required:"false" format:"uuid" pattern:"^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$"`
	Tipo       string `query:"tipo" required:"false" enum:"entrada,salida,ajuste"`
}
type SolicitudMovimiento struct {
	ProductoID      string `json:"producto_id" format:"uuid" pattern:"^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$" example:"11111111-1111-4111-8111-111111111111"`
	Tipo            string `json:"tipo" enum:"entrada,salida,ajuste" example:"entrada"`
	Cantidad        int64  `json:"cantidad" minimum:"0" maximum:"1000000000" example:"5" doc:"Para entradas/salidas, mayor que cero. Para ajustes, nuevo saldo absoluto."`
	VersionEsperada *int64 `json:"version_esperada,omitempty" minimum:"0" doc:"Obligatoria para ajustes: versión de la existencia consultada."`
	Motivo          string `json:"motivo" minLength:"3" maxLength:"500" example:"Reposición de mercadería"`
	Referencia      string `json:"referencia,omitempty" maxLength:"200" example:"OC-2026-001"`
}
type EntradaMovimiento struct {
	Clave string `header:"Idempotency-Key" required:"true" minLength:"8" maxLength:"128" pattern:"^[A-Za-z0-9._:-]+$" example:"ingreso-oc-2026-001"`
	Body  SolicitudMovimiento
}
type SalidaExistencia struct{ Body Existencia }
type PaginaExistencias struct {
	Datos []Existencia `json:"datos"`
	Total int64        `json:"total" minimum:"0"`
}
type SalidaExistencias struct{ Body PaginaExistencias }
type PaginaMovimientos struct {
	Datos []Movimiento `json:"datos"`
	Total int64        `json:"total" minimum:"0"`
}
type SalidaMovimientos struct{ Body PaginaMovimientos }
type SalidaMovimiento struct{ Body Movimiento }
type SalidaSalud struct {
	Body struct {
		Estado  string `json:"estado" example:"ok"`
		Version string `json:"version" example:"a1b2c3d"`
	}
}
