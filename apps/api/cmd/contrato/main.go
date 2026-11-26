package main

import (
	"github.com/pingeso/stock/apps/api/internal/api"
	"os"
)

func main() {
	_, a := api.Nuevo(nil, api.Opciones{Ambiente: "dev", Cookie: "sesion"})
	datos, err := a.OpenAPI().YAML()
	if err != nil {
		panic(err)
	}
	if _, err = os.Stdout.Write(datos); err != nil {
		panic(err)
	}
}
