package nrpq

import (
	"testing"

	"github.com/dave/dst"
	"github.com/newrelic/go-easy-instrumentation/internal/sqlhelpers"
	"github.com/newrelic/go-easy-instrumentation/parser"
	"github.com/stretchr/testify/assert"
)

func TestInstrumentPQHandler(t *testing.T) {
	tests := []struct {
		name   string
		code   string
		expect string
	}{
		{
			name: "instrument QueryRow",
			code: `package main

import (
	"database/sql"
	"fmt"
	_ "github.com/lib/pq"
)

func main() {
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
`,
			expect: `package main

import (
	"context"
	"database/sql"
	"fmt"

	_ "github.com/newrelic/go-agent/v3/integrations/nrpq"
	"github.com/newrelic/go-agent/v3/newrelic"
)

func main() {
	db, err := sql.Open("nrpq", "postgres://postgres:password@localhost/postgres?sslmode=disable")
	if err != nil {
		panic(err)
	}
	defer db.Close()
	nrTxn := NewRelicAgent.StartTransaction("postgres/QueryRow")
	ctx := newrelic.NewContext(context.Background(), nrTxn)
	row := db.QueryRowContext(ctx, "SELECT version()")
	var version string
	err = row.Scan(&version)
	nrTxn.End()
	if err != nil {
		panic(err)
	}
	fmt.Println(version)
}
`,
		},
		{
			name: "instrument Exec",
			code: `package main

import (
	"database/sql"
	_ "github.com/lib/pq"
)

func main() {
	db, err := sql.Open("postgres", "postgres://postgres:password@localhost/postgres?sslmode=disable")
	if err != nil {
		panic(err)
	}
	defer db.Close()
	result, err := db.Exec("INSERT INTO users(name) VALUES($1)", "alice")
	if err != nil {
		panic(err)
	}
	_ = result
}
`,
			expect: `package main

import (
	"context"
	"database/sql"

	_ "github.com/newrelic/go-agent/v3/integrations/nrpq"
	"github.com/newrelic/go-agent/v3/newrelic"
)

func main() {
	db, err := sql.Open("nrpq", "postgres://postgres:password@localhost/postgres?sslmode=disable")
	if err != nil {
		panic(err)
	}
	defer db.Close()
	nrTxn := NewRelicAgent.StartTransaction("postgres/Exec")
	ctx := newrelic.NewContext(context.Background(), nrTxn)
	result, err := db.ExecContext(ctx, "INSERT INTO users(name) VALUES($1)", "alice")
	if err != nil {
		panic(err)
	}
	_ = result
	nrTxn.End()
}
`,
		},
		{
			name: "swap driver in initDB helper, no transaction wrap (queries elsewhere)",
			code: `package main

import (
	"database/sql"
	_ "github.com/lib/pq"
)

func initDB() (*sql.DB, error) {
	db, err := sql.Open("postgres", "postgres://localhost/x")
	return db, err
}

func main() {
	db, err := initDB()
	if err != nil {
		panic(err)
	}
	_ = db
}
`,
			expect: `package main

import (
	"database/sql"
	_ "github.com/newrelic/go-agent/v3/integrations/nrpq"
)

func initDB() (*sql.DB, error) {
	db, err := sql.Open("nrpq", "postgres://localhost/x")
	return db, err
}

func main() {
	db, err := initDB()
	if err != nil {
		panic(err)
	}
	_ = db
}
`,
		},
		{
			name: "already instrumented main is skipped",
			code: `package main

import (
	"database/sql"
	_ "github.com/newrelic/go-agent/v3/integrations/nrpq"
)

func main() {
	db, err := sql.Open("nrpq", "postgres://localhost/x")
	if err != nil {
		panic(err)
	}
	_ = db
}
`,
			expect: `package main

import (
	"database/sql"
	_ "github.com/newrelic/go-agent/v3/integrations/nrpq"
)

func main() {
	db, err := sql.Open("nrpq", "postgres://localhost/x")
	if err != nil {
		panic(err)
	}
	_ = db
}
`,
		},
		{
			name: "blank-identifier DB var still gets driver swap, no transaction wrap",
			code: `package main

import (
	"database/sql"
	_ "github.com/lib/pq"
)

func main() {
	_, err := sql.Open("postgres", "postgres://localhost/x")
	if err != nil {
		panic(err)
	}
}
`,
			expect: `package main

import (
	"database/sql"
	_ "github.com/newrelic/go-agent/v3/integrations/nrpq"
)

func main() {
	_, err := sql.Open("nrpq", "postgres://localhost/x")
	if err != nil {
		panic(err)
	}
}
`,
		},
		{
			name: "non-postgres driver is left alone",
			code: `package main

import "database/sql"

func main() {
	db, err := sql.Open("mysql", "user@/db")
	if err != nil {
		panic(err)
	}
	_ = db
}
`,
			expect: `package main

import "database/sql"

func main() {
	db, err := sql.Open("mysql", "user@/db")
	if err != nil {
		panic(err)
	}
	_ = db
}
`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer parser.PanicRecovery(t)
			got := parser.RunStatelessTracingFunction(t, tt.code, InstrumentPQHandler)
			assert.Equal(t, tt.expect, got)
		})
	}
}

func TestScanForPostgresOpen(t *testing.T) {
	openStmt := func(driver string) dst.Stmt {
		return &dst.AssignStmt{
			Lhs: []dst.Expr{&dst.Ident{Name: "db"}, &dst.Ident{Name: "err"}},
			Rhs: []dst.Expr{
				&dst.CallExpr{
					Fun: &dst.Ident{Name: "Open", Path: sqlhelpers.SQLImportPath},
					Args: []dst.Expr{
						&dst.BasicLit{Value: driver},
						&dst.BasicLit{Value: `"connstr"`},
					},
				},
			},
		}
	}

	tests := []struct {
		name      string
		body      *dst.BlockStmt
		wantState driverState
		wantVar   string
	}{
		{
			name:      "nil body",
			body:      nil,
			wantState: driverNone,
		},
		{
			name:      "empty body",
			body:      &dst.BlockStmt{},
			wantState: driverNone,
		},
		{
			name:      "postgres driver detected",
			body:      &dst.BlockStmt{List: []dst.Stmt{openStmt(`"postgres"`)}},
			wantState: driverPostgres,
			wantVar:   "db",
		},
		{
			name:      "nrpq driver detected as already instrumented",
			body:      &dst.BlockStmt{List: []dst.Stmt{openStmt(`"nrpq"`)}},
			wantState: driverAlreadyNRPQ,
			wantVar:   "db",
		},
		{
			name:      "mysql driver is not a match",
			body:      &dst.BlockStmt{List: []dst.Stmt{openStmt(`"mysql"`)}},
			wantState: driverNone,
		},
		{
			name: "first match wins (postgres before nrpq)",
			body: &dst.BlockStmt{List: []dst.Stmt{
				openStmt(`"postgres"`),
				openStmt(`"nrpq"`),
			}},
			wantState: driverPostgres,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := scanForPostgresOpen(tt.body)
			assert.Equal(t, tt.wantState, got.state)
			if tt.wantVar != "" {
				assert.Equal(t, tt.wantVar, got.dbVar)
			}
		})
	}
}
