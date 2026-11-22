// Command codegen genera contracts/openapi.yaml a partir del código del servicio.
package main

import (
	"fmt"
	"os"
	"path/filepath"

	"stock/internal/api"
)

const destinoPorDefecto = "../../contracts/openapi.yaml"

func main() {
	destino := destinoPorDefecto
	if len(os.Args) > 1 {
		destino = os.Args[1]
	}

	if err := escribirContrato(destino); err != nil {
		fmt.Fprintln(os.Stderr, "codegen:", err)
		os.Exit(1)
	}
}

func escribirContrato(destino string) error {
	contenido, err := api.GenerarContrato()
	if err != nil {
		return fmt.Errorf("generar el contrato: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(destino), 0o750); err != nil {
		return fmt.Errorf("crear la carpeta del contrato: %w", err)
	}
	return os.WriteFile(destino, contenido, 0o600)
}
