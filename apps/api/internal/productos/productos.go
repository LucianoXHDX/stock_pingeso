package productos

import (
	"context"
	"github.com/google/uuid"
	"net/http"
	"strings"
	"time"

	"github.com/pingeso/stock/apps/api/internal/auth"
	"github.com/pingeso/stock/apps/api/internal/service"
)

type Catalogo struct {
	baseURL string
	cliente *http.Client
}

func Nuevo(baseURL string) *Catalogo {
	return &Catalogo{baseURL: strings.TrimRight(baseURL, "/"), cliente: &http.Client{Timeout: 5 * time.Second, CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }}}
}

func (c *Catalogo) Existe(ctx context.Context, id string) error {
	cliente, err := NewClientWithResponses(c.baseURL, WithHTTPClient(c.cliente), WithRequestEditorFn(func(ctx context.Context, r *http.Request) error {
		r.Header.Set("Authorization", "Bearer "+auth.Credencial(ctx))
		r.Header.Set("X-Request-Id", auth.RequestID(ctx))
		return nil
	}))
	if err != nil {
		return service.ErrProveedor
	}
	productoID, err := uuid.Parse(id)
	if err != nil {
		return service.ErrInvalido
	}
	respuesta, err := cliente.ObtenerProductoWithResponse(ctx, productoID, &ObtenerProductoParams{})
	if err != nil {
		return service.ErrProveedor
	}
	if respuesta.StatusCode() == http.StatusNotFound {
		return service.ErrNoEncontrado
	}
	if respuesta.StatusCode() != http.StatusOK || respuesta.JSON200 == nil || !strings.EqualFold(respuesta.JSON200.Id.String(), id) {
		return service.ErrProveedor
	}
	return nil
}
