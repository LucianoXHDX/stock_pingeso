// Package service contiene las reglas de STOCK, sin dependencias de HTTP ni SQL.
package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"
)

const CantidadMaxima int64 = 1_000_000_000

var (
	ErrNoEncontrado = errors.New("recurso no encontrado")
	ErrSinStock     = errors.New("stock insuficiente")
	ErrConflicto    = errors.New("clave de idempotencia reutilizada con otro contenido")
	ErrVersion      = errors.New("la existencia cambió; consulte su versión actual")
	ErrProveedor    = errors.New("el catálogo de Productos no está disponible")
	ErrInvalido     = errors.New("solicitud inválida")
)

type Existencia struct {
	ProductoID    string    `json:"producto_id"`
	Cantidad      int64     `json:"cantidad"`
	Version       int64     `json:"version"`
	ActualizadoEn time.Time `json:"actualizado_en"`
}

type Movimiento struct {
	ID             string    `json:"id"`
	ProductoID     string    `json:"producto_id"`
	Tipo           string    `json:"tipo"`
	Cantidad       int64     `json:"cantidad"`
	SaldoAnterior  int64     `json:"saldo_anterior"`
	SaldoPosterior int64     `json:"saldo_posterior"`
	Motivo         string    `json:"motivo"`
	Referencia     string    `json:"referencia"`
	ActorID        string    `json:"actor_id"`
	CreadoEn       time.Time `json:"creado_en"`
}

// En un ajuste Cantidad es el saldo final deseado, incluso si es cero.
// En el movimiento resultante Cantidad siempre expresa la variación absoluta.
type SolicitudMovimiento struct {
	ProductoID      string `json:"producto_id"`
	Tipo            string `json:"tipo"`
	Cantidad        int64  `json:"cantidad"`
	VersionEsperada *int64 `json:"version_esperada,omitempty"`
	Motivo          string `json:"motivo"`
	Referencia      string `json:"referencia"`
}

type Filtro struct {
	ProductoID     string
	Tipo           string
	Limite         int
	Desplazamiento int
}

type ResultadoMovimientos struct {
	Datos []Movimiento `json:"datos"`
	Total int64        `json:"total"`
}

type ResultadoExistencias struct {
	Datos []Existencia `json:"datos"`
	Total int64        `json:"total"`
}

// Almacen ejecuta la función de dominio con la existencia bloqueada y persiste
// su resultado junto con auditoría e idempotencia, en una sola transacción.
type Almacen interface {
	Consultar(context.Context, string) (Existencia, error)
	Listar(context.Context, Filtro) (ResultadoExistencias, error)
	Historial(context.Context, Filtro) (ResultadoMovimientos, error)
	BuscarMovimiento(context.Context, string, string) (Movimiento, string, error)
	Ejecutar(context.Context, string, string, string, string, func(Existencia) (Movimiento, error)) (Movimiento, error)
}

type Catalogo interface {
	Existe(context.Context, string) error
}

type Servicio struct {
	almacen  Almacen
	catalogo Catalogo
}

func Nuevo(almacen Almacen, catalogo Catalogo) *Servicio {
	return &Servicio{almacen: almacen, catalogo: catalogo}
}

func (s *Servicio) Consultar(ctx context.Context, productoID string) (Existencia, error) {
	id, err := normalizarUUID(productoID)
	if err != nil {
		return Existencia{}, err
	}
	existencia, err := s.almacen.Consultar(ctx, id)
	if !errors.Is(err, ErrNoEncontrado) {
		return existencia, err
	}
	if err := s.catalogo.Existe(ctx, id); err != nil {
		return Existencia{}, err
	}
	return Existencia{ProductoID: id}, nil
}

func (s *Servicio) Listar(ctx context.Context, filtro Filtro) (ResultadoExistencias, error) {
	filtro, err := normalizarFiltro(filtro)
	if err != nil {
		return ResultadoExistencias{}, err
	}
	if filtro.Tipo != "" {
		return ResultadoExistencias{}, fmt.Errorf("%w: el listado de existencias no filtra por tipo", ErrInvalido)
	}
	return s.almacen.Listar(ctx, filtro)
}

func (s *Servicio) Historial(ctx context.Context, filtro Filtro) (ResultadoMovimientos, error) {
	filtro, err := normalizarFiltro(filtro)
	if err != nil {
		return ResultadoMovimientos{}, err
	}
	return s.almacen.Historial(ctx, filtro)
}

func (s *Servicio) Mover(ctx context.Context, actorID, clave string, solicitud SolicitudMovimiento) (Movimiento, error) {
	actorID, solicitud, err := validarMovimiento(actorID, clave, solicitud)
	if err != nil {
		return Movimiento{}, err
	}
	huella, err := huellaSolicitud(solicitud)
	if err != nil {
		return Movimiento{}, err
	}
	// Un reintento confirmado debe poder responder aunque Productos esté caído.
	anterior, huellaAnterior, err := s.almacen.BuscarMovimiento(ctx, actorID, clave)
	if err == nil {
		if huellaAnterior != huella {
			return Movimiento{}, ErrConflicto
		}
		return anterior, nil
	}
	if !errors.Is(err, ErrNoEncontrado) {
		return Movimiento{}, err
	}
	if err := s.catalogo.Existe(ctx, solicitud.ProductoID); err != nil {
		return Movimiento{}, err
	}
	return s.almacen.Ejecutar(ctx, actorID, clave, huella, solicitud.ProductoID, func(actual Existencia) (Movimiento, error) {
		return construirMovimiento(actorID, solicitud, actual)
	})
}

func construirMovimiento(actorID string, solicitud SolicitudMovimiento, actual Existencia) (Movimiento, error) {
	saldo := actual.Cantidad
	switch solicitud.Tipo {
	case "entrada":
		if solicitud.Cantidad > CantidadMaxima-saldo {
			return Movimiento{}, fmt.Errorf("%w: el saldo excedería el máximo permitido", ErrInvalido)
		}
		saldo += solicitud.Cantidad
	case "salida":
		if solicitud.Cantidad > saldo {
			return Movimiento{}, ErrSinStock
		}
		saldo -= solicitud.Cantidad
	case "ajuste":
		if *solicitud.VersionEsperada != actual.Version {
			return Movimiento{}, ErrVersion
		}
		saldo = solicitud.Cantidad
		if saldo == actual.Cantidad {
			return Movimiento{}, fmt.Errorf("%w: el ajuste debe cambiar la cantidad", ErrInvalido)
		}
	}
	cantidad := saldo - actual.Cantidad
	if cantidad < 0 {
		cantidad = -cantidad
	}
	id, err := nuevoUUID()
	if err != nil {
		return Movimiento{}, err
	}
	return Movimiento{
		ID: id, ProductoID: solicitud.ProductoID, Tipo: solicitud.Tipo,
		Cantidad: cantidad, SaldoAnterior: actual.Cantidad, SaldoPosterior: saldo,
		Motivo: solicitud.Motivo, Referencia: solicitud.Referencia, ActorID: actorID,
		CreadoEn: time.Now().UTC().Truncate(time.Microsecond),
	}, nil
}

func validarMovimiento(actorID, clave string, solicitud SolicitudMovimiento) (string, SolicitudMovimiento, error) {
	actor, err := normalizarUUID(actorID)
	if err != nil {
		return "", solicitud, err
	}
	producto, err := normalizarUUID(solicitud.ProductoID)
	if err != nil {
		return "", solicitud, err
	}
	if len(clave) < 8 || len(clave) > 128 {
		return "", solicitud, fmt.Errorf("%w: la clave de idempotencia debe tener entre 8 y 128 caracteres", ErrInvalido)
	}
	for _, caracter := range clave {
		if caracter < 33 || caracter > 126 {
			return "", solicitud, fmt.Errorf("%w: clave de idempotencia inválida", ErrInvalido)
		}
	}
	solicitud.ProductoID = producto
	solicitud.Motivo = strings.TrimSpace(solicitud.Motivo)
	solicitud.Referencia = strings.TrimSpace(solicitud.Referencia)
	if !utf8.ValidString(solicitud.Motivo) || !utf8.ValidString(solicitud.Referencia) ||
		strings.ContainsRune(solicitud.Motivo, 0) || strings.ContainsRune(solicitud.Referencia, 0) ||
		utf8.RuneCountInString(solicitud.Motivo) < 3 || utf8.RuneCountInString(solicitud.Motivo) > 500 ||
		utf8.RuneCountInString(solicitud.Referencia) > 200 {
		return "", solicitud, fmt.Errorf("%w: motivo o referencia fuera de los límites", ErrInvalido)
	}
	if !tipoValido(solicitud.Tipo) || solicitud.Cantidad < 0 || solicitud.Cantidad > CantidadMaxima ||
		(solicitud.Tipo != "ajuste" && solicitud.Cantidad == 0) {
		return "", solicitud, fmt.Errorf("%w: tipo o cantidad no permitidos", ErrInvalido)
	}
	if solicitud.Tipo == "ajuste" && (solicitud.VersionEsperada == nil || *solicitud.VersionEsperada < 0) {
		return "", solicitud, fmt.Errorf("%w: el ajuste requiere una versión esperada no negativa", ErrInvalido)
	}
	return actor, solicitud, nil
}

func normalizarFiltro(filtro Filtro) (Filtro, error) {
	if filtro.ProductoID != "" {
		id, err := normalizarUUID(filtro.ProductoID)
		if err != nil {
			return filtro, err
		}
		filtro.ProductoID = id
	}
	if filtro.Tipo != "" && !tipoValido(filtro.Tipo) {
		return filtro, fmt.Errorf("%w: tipo de movimiento desconocido", ErrInvalido)
	}
	if filtro.Limite == 0 {
		filtro.Limite = 20
	}
	if filtro.Limite < 1 || filtro.Limite > 100 || filtro.Desplazamiento < 0 || filtro.Desplazamiento > 1_000_000 {
		return filtro, fmt.Errorf("%w: paginación fuera de los límites", ErrInvalido)
	}
	return filtro, nil
}

func tipoValido(tipo string) bool {
	return tipo == "entrada" || tipo == "salida" || tipo == "ajuste"
}

func normalizarUUID(valor string) (string, error) {
	if len(valor) != 36 || valor[8] != '-' || valor[13] != '-' || valor[18] != '-' || valor[23] != '-' {
		return "", fmt.Errorf("%w: se requiere un UUID", ErrInvalido)
	}
	digitos := strings.ReplaceAll(valor, "-", "")
	if len(digitos) != 32 {
		return "", fmt.Errorf("%w: se requiere un UUID", ErrInvalido)
	}
	if _, err := hex.DecodeString(digitos); err != nil {
		return "", fmt.Errorf("%w: se requiere un UUID", ErrInvalido)
	}
	return strings.ToLower(valor), nil
}

func huellaSolicitud(solicitud SolicitudMovimiento) (string, error) {
	datos, err := json.Marshal(solicitud)
	if err != nil {
		return "", err
	}
	huella := sha256.Sum256(datos)
	return hex.EncodeToString(huella[:]), nil
}

func nuevoUUID() (string, error) {
	var bytes [16]byte
	if _, err := rand.Read(bytes[:]); err != nil {
		return "", fmt.Errorf("generar identificador de movimiento: %w", err)
	}
	bytes[6] = (bytes[6] & 0x0f) | 0x40
	bytes[8] = (bytes[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", bytes[0:4], bytes[4:6], bytes[6:8], bytes[8:10], bytes[10:16]), nil
}
