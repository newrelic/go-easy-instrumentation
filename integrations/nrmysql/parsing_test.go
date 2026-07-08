package nrmysql_test

import (
	"testing"

	"github.com/newrelic/go-easy-instrumentation/integrations/nrmysql"
	"github.com/newrelic/go-easy-instrumentation/parser"
	"github.com/stretchr/testify/assert"
)

func TestInstrumentSQLHandler(t *testing.T) {
	tests := []struct {
		name   string
		code   string
		expect string
	}{
		{
			name: "instrument QueryRow",
			code: `package main

import (
	"github.com/newrelic/go-easy-instrumentation/integrations/nrmysql"
	"github.com/newrelic/go-easy-instrumentation/parser"
	"database/sql"
	_ "github.com/go-sql-driver/mysql"
)

func main() {
	db, err := sql.Open("nrmysql", "root@/information_schema")
	if err != nil {
		panic(err)
	}
	defer db.Close()

	row := db.QueryRow("SELECT * FROM users")
}
`,
			expect: `package main

import (
	"context"
	"database/sql"

	_ "github.com/go-sql-driver/mysql"
)

func main() {
	db, err := sql.Open("nrmysql", "root@/information_schema")
	if err != nil {
		panic(err)
	}
	defer db.Close()

	nrTxn := NewRelicAgent.StartTransaction("mySQL/QueryRow")
	ctx := newrelic.NewContext(context.Background(), nrTxn)

	row := db.QueryRowContext(ctx, "SELECT * FROM users")
	nrTxn.End()
}
`,
		},
		{
			name: "instrument Query with loop",
			code: `package main

import (
	"database/sql"
	_ "github.com/go-sql-driver/mysql"
)

func main() {
	db, err := sql.Open("nrmysql", "root@/information_schema")
	if err != nil {
		panic(err)
	}
	defer db.Close()

	rows, err := db.QueryRow("SELECT * FROM users")
	if err != nil {
		panic(err)
	}
	defer rows.Close()

	for rows.Next() {
		var id int
		var name string
		err := rows.Scan(&id, &name)
		if err != nil {
			panic(err)
		}
	}
}
`,
			expect: `package main

import (
	"context"
	"database/sql"

	_ "github.com/go-sql-driver/mysql"
)

func main() {
	db, err := sql.Open("nrmysql", "root@/information_schema")
	if err != nil {
		panic(err)
	}
	defer db.Close()

	nrTxn := NewRelicAgent.StartTransaction("mySQL/QueryRow")
	ctx := newrelic.NewContext(context.Background(), nrTxn)

	rows, err := db.QueryRowContext(ctx, "SELECT * FROM users")
	if err != nil {
		panic(err)
	}
	defer rows.Close()

	for rows.Next() {
		var id int
		var name string
		err := rows.Scan(&id, &name)
		if err != nil {
			panic(err)
		}
	}
	nrTxn.End()
}
`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer parser.PanicRecovery(t)
			got := parser.RunStatelessTracingFunction(t, tt.code, nrmysql.InstrumentSQLHandler)
			assert.Equal(t, tt.expect, got)
		})
	}
}
