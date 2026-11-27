package auth

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/MicahParks/keyfunc/v3"
	"github.com/golang-jwt/jwt/v5"
)

type claveContexto uint8

const claveIdentidad claveContexto = iota
const claveCredencial claveContexto = iota + 1
const claveRequest claveContexto = iota + 2

type Identidad struct {
	ID       string
	Permisos []string
}

type Claims struct {
	Permisos []string `json:"permisos"`
	jwt.RegisteredClaims
}

type Validador struct {
	claves            keyfunc.Keyfunc
	emisor, audiencia string
}

func Nuevo(ctx context.Context, jwks, emisor, audiencia string) (*Validador, error) {
	fallarInicio := false
	claves, err := keyfunc.NewDefaultOverrideCtx(ctx, []string{jwks}, keyfunc.Override{Client: &http.Client{Timeout: 5 * time.Second}, HTTPTimeout: 5 * time.Second, RateLimitWaitMax: time.Second, NoErrorReturnFirstHTTPReq: &fallarInicio})
	if err != nil {
		return nil, fmt.Errorf("no se pudo cargar JWKS")
	}
	return &Validador{claves: claves, emisor: emisor, audiencia: audiencia}, nil
}

func (v *Validador) Validar(ctx context.Context, credencial string) (Identidad, error) {
	claims := &Claims{}
	contexto, cancelar := context.WithTimeout(ctx, 5*time.Second)
	defer cancelar()
	token, err := jwt.ParseWithClaims(credencial, claims, v.claves.KeyfuncCtx(contexto), jwt.WithValidMethods([]string{"RS256", "EdDSA"}), jwt.WithIssuer(v.emisor), jwt.WithAudience(v.audiencia), jwt.WithExpirationRequired(), jwt.WithIssuedAt())
	if err != nil || token == nil || !token.Valid || !EsUUID(claims.Subject) {
		return Identidad{}, fmt.Errorf("sesión inválida o expirada")
	}
	return Identidad{ID: strings.ToLower(claims.Subject), Permisos: claims.Permisos}, nil
}

func (i Identidad) Tiene(permiso string) bool {
	for _, actual := range i.Permisos {
		if actual == permiso {
			return true
		}
	}
	return false
}

func ConIdentidad(ctx context.Context, identidad Identidad, credencial string) context.Context {
	return context.WithValue(context.WithValue(ctx, claveIdentidad, identidad), claveCredencial, credencial)
}
func Usuario(ctx context.Context) Identidad {
	identidad, _ := ctx.Value(claveIdentidad).(Identidad)
	return identidad
}
func Credencial(ctx context.Context) string {
	valor, _ := ctx.Value(claveCredencial).(string)
	return valor
}
func ConRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, claveRequest, id)
}
func RequestID(ctx context.Context) string { id, _ := ctx.Value(claveRequest).(string); return id }

func EsUUID(valor string) bool {
	if len(valor) != 36 {
		return false
	}
	for i, c := range valor {
		if i == 8 || i == 13 || i == 18 || i == 23 {
			if c != '-' {
				return false
			}
			continue
		}
		if !(c >= '0' && c <= '9') && !(c >= 'a' && c <= 'f') && !(c >= 'A' && c <= 'F') {
			return false
		}
	}
	return valor != "00000000-0000-0000-0000-000000000000"
}
