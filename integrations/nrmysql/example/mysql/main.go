// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"database/sql"
	"fmt"

	_ "github.com/go-sql-driver/mysql"
)

func main() {
	// Set up a local mysql docker container with:
	// docker run -it -p 3306:3306 --net "bridge" -e MYSQL_ALLOW_EMPTY_PASSWORD=true mysql

	db, err := sql.Open("mysql", "root@/information_schema")
	if err != nil {
		panic(err)
	}

	row := db.QueryRow("SELECT count(*) from tables")
	var count int
	row.Scan(&count)

	fmt.Println("number of tables in information_schema", count)
}
