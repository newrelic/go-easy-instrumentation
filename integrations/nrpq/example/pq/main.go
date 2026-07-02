package main

import (
	"database/sql"
	"fmt"
	_ "github.com/lib/pq"
)

func main() {
	// Set up a local postgres docker container with:
	// docker run -it -p 5432:5432 -e POSTGRES_PASSWORD=password postgres
	db, err := sql.Open("postgres", "postgres://postgres:password@localhost/postgres?sslmode=disable")
	if err != nil {
		panic(err)
	}
	defer db.Close()

	row := db.QueryRow("SELECT version()")
	var version string
	err = row.Scan(&version)
	if err != nil {
		panic(err)
	}
	fmt.Println(version)
}
