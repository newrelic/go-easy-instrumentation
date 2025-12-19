package parser

import (
	"go/token"
	"testing"

	"github.com/dave/dst"
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
			defer panicRecovery(t)
			got := testStatelessTracingFunction(t, tt.code, InstrumentSQLHandler)
			assert.Equal(t, tt.expect, got)
		})
	}
}

func TestDetectSQLOpenCall(t *testing.T) {
	tests := []struct {
		name string
		stmt dst.Stmt
		want string
	}{
		{
			name: "detect sql.Open call",
			stmt: &dst.AssignStmt{
				Lhs: []dst.Expr{
					&dst.Ident{Name: "db"},
					&dst.Ident{Name: "err"},
				},
				Tok: token.DEFINE,
				Rhs: []dst.Expr{
					&dst.CallExpr{
						Fun: &dst.Ident{
							Name: "Open",
							Path: sqlImportPath,
						},
						Args: []dst.Expr{
							&dst.BasicLit{Value: `"nrmysql"`},
							&dst.BasicLit{Value: `"root@/information_schema"`},
						},
					},
				},
			},
			want: "db",
		},
		{
			name: "incorrect import path",
			stmt: &dst.AssignStmt{
				Lhs: []dst.Expr{
					&dst.Ident{Name: "db"},
					&dst.Ident{Name: "err"},
				},
				Tok: token.DEFINE,
				Rhs: []dst.Expr{
					&dst.CallExpr{
						Fun: &dst.Ident{
							Name: "Open",
							Path: "some/other/package",
						},
					},
				},
			},
			want: "",
		},
		{
			name: "incorrect function name",
			stmt: &dst.AssignStmt{
				Lhs: []dst.Expr{
					&dst.Ident{Name: "db"},
				},
				Tok: token.DEFINE,
				Rhs: []dst.Expr{
					&dst.CallExpr{
						Fun: &dst.Ident{
							Name: "Connect",
							Path: sqlImportPath,
						},
					},
				},
			},
			want: "",
		},
		{
			name: "not an assignment statement",
			stmt: &dst.ExprStmt{
				X: &dst.CallExpr{
					Fun: &dst.Ident{Name: "Open"},
				},
			},
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer panicRecovery(t)
			got := detectSQLOpenCall(tt.stmt)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestDetectSQLExecutionCall(t *testing.T) {
	tests := []struct {
		name   string
		stmt   dst.Stmt
		dbName string
		want   string
	}{
		{
			name: "detect QueryRow call",
			stmt: &dst.AssignStmt{
				Lhs: []dst.Expr{
					&dst.Ident{Name: "row"},
				},
				Tok: token.DEFINE,
				Rhs: []dst.Expr{
					&dst.CallExpr{
						Fun: &dst.SelectorExpr{
							X:   &dst.Ident{Name: "db"},
							Sel: &dst.Ident{Name: "QueryRow"},
						},
						Args: []dst.Expr{
							&dst.BasicLit{Value: `"SELECT * FROM users"`},
						},
					},
				},
			},
			dbName: "db",
			want:   "QueryRow",
		},
		{
			name: "detect Query call",
			stmt: &dst.AssignStmt{
				Lhs: []dst.Expr{
					&dst.Ident{Name: "rows"},
					&dst.Ident{Name: "err"},
				},
				Tok: token.DEFINE,
				Rhs: []dst.Expr{
					&dst.CallExpr{
						Fun: &dst.SelectorExpr{
							X:   &dst.Ident{Name: "db"},
							Sel: &dst.Ident{Name: "Query"},
						},
					},
				},
			},
			dbName: "db",
			want:   "Query",
		},
		{
			name: "detect Exec call",
			stmt: &dst.AssignStmt{
				Lhs: []dst.Expr{
					&dst.Ident{Name: "result"},
					&dst.Ident{Name: "err"},
				},
				Tok: token.DEFINE,
				Rhs: []dst.Expr{
					&dst.CallExpr{
						Fun: &dst.SelectorExpr{
							X:   &dst.Ident{Name: "db"},
							Sel: &dst.Ident{Name: "Exec"},
						},
					},
				},
			},
			dbName: "db",
			want:   "Exec",
		},
		{
			name: "wrong DB variable name",
			stmt: &dst.AssignStmt{
				Lhs: []dst.Expr{
					&dst.Ident{Name: "row"},
				},
				Tok: token.DEFINE,
				Rhs: []dst.Expr{
					&dst.CallExpr{
						Fun: &dst.SelectorExpr{
							X:   &dst.Ident{Name: "database"},
							Sel: &dst.Ident{Name: "QueryRow"},
						},
					},
				},
			},
			dbName: "db",
			want:   "",
		},
		{
			name: "unsupported method",
			stmt: &dst.AssignStmt{
				Lhs: []dst.Expr{
					&dst.Ident{Name: "result"},
				},
				Tok: token.DEFINE,
				Rhs: []dst.Expr{
					&dst.CallExpr{
						Fun: &dst.SelectorExpr{
							X:   &dst.Ident{Name: "db"},
							Sel: &dst.Ident{Name: "Ping"},
						},
					},
				},
			},
			dbName: "db",
			want:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer panicRecovery(t)
			got := detectSQLExecutionCall(tt.stmt, tt.dbName)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestReplaceSQLMethodWithContext(t *testing.T) {
	tests := []struct {
		name    string
		stmt    dst.Stmt
		ctxName string
		want    string
	}{
		{
			name: "replace QueryRow with QueryRowContext",
			stmt: &dst.AssignStmt{
				Lhs: []dst.Expr{
					&dst.Ident{Name: "row"},
				},
				Tok: token.DEFINE,
				Rhs: []dst.Expr{
					&dst.CallExpr{
						Fun: &dst.SelectorExpr{
							X:   &dst.Ident{Name: "db"},
							Sel: &dst.Ident{Name: "QueryRow"},
						},
						Args: []dst.Expr{
							&dst.BasicLit{Value: `"SELECT * FROM users"`},
						},
					},
				},
			},
			ctxName: "ctx",
			want:    "QueryRowContext",
		},
		{
			name: "replace Query with QueryContext",
			stmt: &dst.AssignStmt{
				Lhs: []dst.Expr{
					&dst.Ident{Name: "rows"},
				},
				Tok: token.DEFINE,
				Rhs: []dst.Expr{
					&dst.CallExpr{
						Fun: &dst.SelectorExpr{
							X:   &dst.Ident{Name: "db"},
							Sel: &dst.Ident{Name: "Query"},
						},
						Args: []dst.Expr{
							&dst.BasicLit{Value: `"SELECT * FROM users"`},
						},
					},
				},
			},
			ctxName: "ctx",
			want:    "QueryContext",
		},
		{
			name: "replace Exec with ExecContext",
			stmt: &dst.AssignStmt{
				Lhs: []dst.Expr{
					&dst.Ident{Name: "result"},
				},
				Tok: token.DEFINE,
				Rhs: []dst.Expr{
					&dst.CallExpr{
						Fun: &dst.SelectorExpr{
							X:   &dst.Ident{Name: "db"},
							Sel: &dst.Ident{Name: "Exec"},
						},
						Args: []dst.Expr{
							&dst.BasicLit{Value: `"INSERT INTO users VALUES (1)"`},
						},
					},
				},
			},
			ctxName: "ctx",
			want:    "ExecContext",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer panicRecovery(t)
			replaceSQLMethodWithContext(tt.stmt, tt.ctxName)

			assignStmt := tt.stmt.(*dst.AssignStmt)
			callExpr := assignStmt.Rhs[0].(*dst.CallExpr)
			selExpr := callExpr.Fun.(*dst.SelectorExpr)

			// Check method name was changed
			assert.Equal(t, tt.want, selExpr.Sel.Name)

			// Check context was prepended as first argument
			assert.Greater(t, len(callExpr.Args), 0)
			if ctxIdent, ok := callExpr.Args[0].(*dst.Ident); ok {
				assert.Equal(t, tt.ctxName, ctxIdent.Name)
			}
		})
	}
}

func TestFindLastUsageOfVariable(t *testing.T) {
	tests := []struct {
		name       string
		stmts      []dst.Stmt
		varName    string
		startIndex int
		want       int
	}{
		{
			name: "variable used in subsequent statement",
			stmts: []dst.Stmt{
				&dst.AssignStmt{
					Lhs: []dst.Expr{&dst.Ident{Name: "row"}},
					Tok: token.DEFINE,
					Rhs: []dst.Expr{&dst.CallExpr{}},
				},
				&dst.ExprStmt{
					X: &dst.CallExpr{
						Fun: &dst.SelectorExpr{
							X:   &dst.Ident{Name: "row"},
							Sel: &dst.Ident{Name: "Scan"},
						},
					},
				},
			},
			varName:    "row",
			startIndex: 0,
			want:       1,
		},
		{
			name: "variable used multiple times",
			stmts: []dst.Stmt{
				&dst.AssignStmt{
					Lhs: []dst.Expr{&dst.Ident{Name: "result"}},
					Tok: token.DEFINE,
					Rhs: []dst.Expr{&dst.CallExpr{}},
				},
				&dst.ExprStmt{
					X: &dst.CallExpr{
						Fun: &dst.SelectorExpr{
							X:   &dst.Ident{Name: "result"},
							Sel: &dst.Ident{Name: "LastInsertId"},
						},
					},
				},
				&dst.ExprStmt{
					X: &dst.CallExpr{
						Fun: &dst.SelectorExpr{
							X:   &dst.Ident{Name: "result"},
							Sel: &dst.Ident{Name: "RowsAffected"},
						},
					},
				},
			},
			varName:    "result",
			startIndex: 0,
			want:       2,
		},
		{
			name: "variable never used after start",
			stmts: []dst.Stmt{
				&dst.AssignStmt{
					Lhs: []dst.Expr{&dst.Ident{Name: "row"}},
					Tok: token.DEFINE,
					Rhs: []dst.Expr{&dst.CallExpr{}},
				},
				&dst.ExprStmt{
					X: &dst.CallExpr{
						Fun: &dst.Ident{Name: "doSomething"},
					},
				},
			},
			varName:    "row",
			startIndex: 0,
			want:       0,
		},
		{
			name: "variable used in nested expression",
			stmts: []dst.Stmt{
				&dst.AssignStmt{
					Lhs: []dst.Expr{&dst.Ident{Name: "rows"}},
					Tok: token.DEFINE,
					Rhs: []dst.Expr{&dst.CallExpr{}},
				},
				&dst.DeferStmt{
					Call: &dst.CallExpr{
						Fun: &dst.SelectorExpr{
							X:   &dst.Ident{Name: "rows"},
							Sel: &dst.Ident{Name: "Close"},
						},
					},
				},
				&dst.ForStmt{
					Cond: &dst.CallExpr{
						Fun: &dst.SelectorExpr{
							X:   &dst.Ident{Name: "rows"},
							Sel: &dst.Ident{Name: "Next"},
						},
					},
					Body: &dst.BlockStmt{
						List: []dst.Stmt{
							&dst.ExprStmt{
								X: &dst.CallExpr{
									Fun: &dst.Ident{Name: "process"},
								},
							},
						},
					},
				},
			},
			varName:    "rows",
			startIndex: 0,
			want:       2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer panicRecovery(t)
			got := findLastUsageOfExecutionResult(tt.stmts, tt.varName, tt.startIndex)
			assert.Equal(t, tt.want, got)
		})
	}
}
