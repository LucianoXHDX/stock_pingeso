package main

import (
	"context"
	"database/sql"
	"errors"
	"flag"
	"fmt"
	"os"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
)

func ejecutar(ctx context.Context, argumentos []string) error {
	banderas := flag.NewFlagSet("goose", flag.ContinueOnError)
	directorio := banderas.String("dir", ".", "directorio de migraciones")
	if err := banderas.Parse(argumentos); err != nil {
		return err
	}
	posicionales := banderas.Args()
	if len(posicionales) < 3 {
		return errors.New("uso: goose -dir <carpeta> postgres <dsn> <comando>")
	}
	dialecto, dsn, comando := posicionales[0], posicionales[1], posicionales[2]
	if dialecto != "postgres" && dialecto != "pgx" {
		return fmt.Errorf("dialecto no soportado: %s", dialecto)
	}
	if err := goose.SetDialect("postgres"); err != nil {
		return err
	}
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return err
	}
	defer db.Close()
	return goose.RunContext(ctx, comando, db, *directorio, posicionales[3:]...)
}

func main() {
	if err := ejecutar(context.Background(), os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
