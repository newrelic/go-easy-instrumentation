package main

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
)

func main() {
	// Set up a local postgres docker container with:
	// docker run -it -p 5432:5432 -e POSTGRES_PASSWORD=password postgres

	conn, err := pgx.Connect(context.Background(), "postgres://postgres:password@localhost/postgres")
	if err != nil {
		panic(err)
	}
	defer conn.Close(context.Background())

	var version string
	err = conn.QueryRow(context.Background(), "SELECT version()").Scan(&version)
	if err != nil {
		panic(err)
	}
	fmt.Println(version)
}
