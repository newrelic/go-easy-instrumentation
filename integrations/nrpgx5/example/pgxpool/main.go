package main

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	// Set up a local postgres docker container with:
	// docker run -it -p 5432:5432 -e POSTGRES_PASSWORD=password postgres

	pool, err := pgxpool.New(context.Background(), "postgres://postgres:password@localhost/postgres")
	if err != nil {
		panic(err)
	}
	defer pool.Close()

	var version string
	err = pool.QueryRow(context.Background(), "SELECT version()").Scan(&version)
	if err != nil {
		panic(err)
	}
	fmt.Println(version)
}
