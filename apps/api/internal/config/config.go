package config

import (
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
)

type Configuracion struct {
	Puerto, Ambiente, BaseDatos, JWKS, Emisor, Audiencia, Productos, Frontend, Cookie string
}

func Cargar() (Configuracion, error) {
	variables := []string{"API_PORT", "API_ENV", "DATABASE_URL", "AUTH_JWKS_URL", "AUTH_ISSUER", "AUTH_AUDIENCE", "PRODUCTOS_URL", "FRONTEND_URL", "TZ"}
	for _, nombre := range variables {
		valor := os.Getenv(nombre)
		if valor == "" || strings.Contains(valor, "replace-with-") {
			return Configuracion{}, fmt.Errorf("configuración obligatoria pendiente: %s", nombre)
		}
	}
	c := Configuracion{Puerto: os.Getenv("API_PORT"), Ambiente: os.Getenv("API_ENV"), BaseDatos: os.Getenv("DATABASE_URL"), JWKS: os.Getenv("AUTH_JWKS_URL"), Emisor: os.Getenv("AUTH_ISSUER"), Audiencia: os.Getenv("AUTH_AUDIENCE"), Productos: os.Getenv("PRODUCTOS_URL"), Frontend: os.Getenv("FRONTEND_URL"), Cookie: os.Getenv("SESSION_COOKIE_NAME")}
	if c.Puerto != "8080" {
		return c, fmt.Errorf("API_PORT debe ser 8080")
	}
	if c.Ambiente != "dev" && c.Ambiente != "test" && c.Ambiente != "prod" {
		return c, fmt.Errorf("API_ENV debe ser dev, test o prod")
	}
	if os.Getenv("TZ") != "America/Santiago" && c.Ambiente != "test" {
		return c, fmt.Errorf("TZ debe ser America/Santiago")
	}
	for nombre, valor := range map[string]string{"AUTH_JWKS_URL": c.JWKS, "PRODUCTOS_URL": c.Productos, "FRONTEND_URL": c.Frontend} {
		u, err := url.Parse(valor)
		if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
			return c, fmt.Errorf("%s debe ser una URL HTTP válida sin credenciales", nombre)
		}
		if c.Ambiente == "prod" && nombre != "PRODUCTOS_URL" && u.Scheme != "https" {
			return c, fmt.Errorf("%s requiere HTTPS en prod", nombre)
		}
		if nombre == "FRONTEND_URL" && u.Path != "" && u.Path != "/" {
			return c, fmt.Errorf("FRONTEND_URL debe ser un origen sin ruta")
		}
	}
	c.Frontend = strings.TrimRight(c.Frontend, "/")
	if c.Cookie == "" {
		c.Cookie = "sesion"
	}
	if err := (&http.Cookie{Name: c.Cookie, Value: "validar"}).Valid(); err != nil {
		return c, fmt.Errorf("SESSION_COOKIE_NAME inválido")
	}
	return c, nil
}
