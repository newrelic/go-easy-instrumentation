package sqlhelpers

import (
	"go/token"
	"testing"

	"github.com/dave/dst"
	"github.com/stretchr/testify/assert"
)

func TestDetectSQLOpen(t *testing.T) {
	tests := []struct {
		name        string
		stmt        dst.Stmt
		wantVar     string
		wantDriver  string // "" means driverArg should be nil
		wantNilArg  bool
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
						Fun: &dst.Ident{Name: "Open", Path: SQLImportPath},
						Args: []dst.Expr{
							&dst.BasicLit{Value: `"nrmysql"`},
							&dst.BasicLit{Value: `"root@/info"`},
						},
					},
				},
			},
			wantVar:    "db",
			wantDriver: `"nrmysql"`,
		},
		{
			name: "detect sql.Open with postgres driver",
			stmt: &dst.AssignStmt{
				Lhs: []dst.Expr{&dst.Ident{Name: "db"}, &dst.Ident{Name: "err"}},
				Tok: token.DEFINE,
				Rhs: []dst.Expr{
					&dst.CallExpr{
						Fun: &dst.Ident{Name: "Open", Path: SQLImportPath},
						Args: []dst.Expr{
							&dst.BasicLit{Value: `"postgres"`},
							&dst.BasicLit{Value: `"postgres://localhost/x"`},
						},
					},
				},
			},
			wantVar:    "db",
			wantDriver: `"postgres"`,
		},
		{
			name: "blank identifier as DB var is preserved",
			stmt: &dst.AssignStmt{
				Lhs: []dst.Expr{&dst.Ident{Name: "_"}, &dst.Ident{Name: "err"}},
				Tok: token.DEFINE,
				Rhs: []dst.Expr{
					&dst.CallExpr{
						Fun: &dst.Ident{Name: "Open", Path: SQLImportPath},
						Args: []dst.Expr{
							&dst.BasicLit{Value: `"postgres"`},
							&dst.BasicLit{Value: `"postgres://localhost/x"`},
						},
					},
				},
			},
			wantVar:    "_",
			wantDriver: `"postgres"`,
		},
		{
			name: "incorrect import path",
			stmt: &dst.AssignStmt{
				Lhs: []dst.Expr{&dst.Ident{Name: "db"}, &dst.Ident{Name: "err"}},
				Tok: token.DEFINE,
				Rhs: []dst.Expr{
					&dst.CallExpr{
						Fun:  &dst.Ident{Name: "Open", Path: "some/other/package"},
						Args: []dst.Expr{&dst.BasicLit{Value: `"x"`}, &dst.BasicLit{Value: `"y"`}},
					},
				},
			},
			wantNilArg: true,
		},
		{
			name: "incorrect function name",
			stmt: &dst.AssignStmt{
				Lhs: []dst.Expr{&dst.Ident{Name: "db"}},
				Tok: token.DEFINE,
				Rhs: []dst.Expr{
					&dst.CallExpr{
						Fun:  &dst.Ident{Name: "Connect", Path: SQLImportPath},
						Args: []dst.Expr{&dst.BasicLit{Value: `"x"`}, &dst.BasicLit{Value: `"y"`}},
					},
				},
			},
			wantNilArg: true,
		},
		{
			name: "wrong arg count",
			stmt: &dst.AssignStmt{
				Lhs: []dst.Expr{&dst.Ident{Name: "db"}, &dst.Ident{Name: "err"}},
				Tok: token.DEFINE,
				Rhs: []dst.Expr{
					&dst.CallExpr{
						Fun:  &dst.Ident{Name: "Open", Path: SQLImportPath},
						Args: []dst.Expr{&dst.BasicLit{Value: `"x"`}},
					},
				},
			},
			wantNilArg: true,
		},
		{
			name: "driver arg is not a string literal",
			stmt: &dst.AssignStmt{
				Lhs: []dst.Expr{&dst.Ident{Name: "db"}, &dst.Ident{Name: "err"}},
				Tok: token.DEFINE,
				Rhs: []dst.Expr{
					&dst.CallExpr{
						Fun: &dst.Ident{Name: "Open", Path: SQLImportPath},
						Args: []dst.Expr{
							&dst.Ident{Name: "driverVar"},
							&dst.BasicLit{Value: `"y"`},
						},
					},
				},
			},
			wantNilArg: true,
		},
		{
			name: "not an assignment statement",
			stmt: &dst.ExprStmt{
				X: &dst.CallExpr{Fun: &dst.Ident{Name: "Open"}},
			},
			wantNilArg: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotVar, gotArg := DetectSQLOpen(tt.stmt)
			assert.Equal(t, tt.wantVar, gotVar)
			if tt.wantNilArg {
				assert.Nil(t, gotArg)
			} else {
				assert.NotNil(t, gotArg)
				assert.Equal(t, tt.wantDriver, gotArg.Value)
			}
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
				Lhs: []dst.Expr{&dst.Ident{Name: "row"}},
				Tok: token.DEFINE,
				Rhs: []dst.Expr{
					&dst.CallExpr{
						Fun: &dst.SelectorExpr{
							X:   &dst.Ident{Name: "db"},
							Sel: &dst.Ident{Name: "QueryRow"},
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
				Lhs: []dst.Expr{&dst.Ident{Name: "rows"}, &dst.Ident{Name: "err"}},
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
				Lhs: []dst.Expr{&dst.Ident{Name: "result"}, &dst.Ident{Name: "err"}},
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
				Lhs: []dst.Expr{&dst.Ident{Name: "row"}},
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
				Lhs: []dst.Expr{&dst.Ident{Name: "result"}},
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
			got := DetectSQLExecutionCall(tt.stmt, tt.dbName)
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
				Lhs: []dst.Expr{&dst.Ident{Name: "row"}},
				Tok: token.DEFINE,
				Rhs: []dst.Expr{
					&dst.CallExpr{
						Fun: &dst.SelectorExpr{
							X:   &dst.Ident{Name: "db"},
							Sel: &dst.Ident{Name: "QueryRow"},
						},
						Args: []dst.Expr{&dst.BasicLit{Value: `"SELECT 1"`}},
					},
				},
			},
			ctxName: "ctx",
			want:    "QueryRowContext",
		},
		{
			name: "replace Query with QueryContext",
			stmt: &dst.AssignStmt{
				Lhs: []dst.Expr{&dst.Ident{Name: "rows"}},
				Tok: token.DEFINE,
				Rhs: []dst.Expr{
					&dst.CallExpr{
						Fun: &dst.SelectorExpr{
							X:   &dst.Ident{Name: "db"},
							Sel: &dst.Ident{Name: "Query"},
						},
						Args: []dst.Expr{&dst.BasicLit{Value: `"SELECT 1"`}},
					},
				},
			},
			ctxName: "ctx",
			want:    "QueryContext",
		},
		{
			name: "replace Exec with ExecContext",
			stmt: &dst.AssignStmt{
				Lhs: []dst.Expr{&dst.Ident{Name: "result"}},
				Tok: token.DEFINE,
				Rhs: []dst.Expr{
					&dst.CallExpr{
						Fun: &dst.SelectorExpr{
							X:   &dst.Ident{Name: "db"},
							Sel: &dst.Ident{Name: "Exec"},
						},
						Args: []dst.Expr{&dst.BasicLit{Value: `"INSERT ..."`}},
					},
				},
			},
			ctxName: "ctx",
			want:    "ExecContext",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ReplaceSQLMethodWithContext(tt.stmt, tt.ctxName)

			assignStmt := tt.stmt.(*dst.AssignStmt)
			callExpr := assignStmt.Rhs[0].(*dst.CallExpr)
			selExpr := callExpr.Fun.(*dst.SelectorExpr)

			assert.Equal(t, tt.want, selExpr.Sel.Name)
			assert.Greater(t, len(callExpr.Args), 0)
			ctxIdent, ok := callExpr.Args[0].(*dst.Ident)
			assert.True(t, ok)
			assert.Equal(t, tt.ctxName, ctxIdent.Name)
		})
	}
}

func TestFindLastUsageOfExecutionResult(t *testing.T) {
	stmt := func(name string) dst.Stmt {
		return &dst.ExprStmt{
			X: &dst.CallExpr{
				Fun: &dst.SelectorExpr{
					X:   &dst.Ident{Name: name},
					Sel: &dst.Ident{Name: "Method"},
				},
			},
		}
	}

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
				stmt("ignored"),
				stmt("row"),
			},
			varName:    "row",
			startIndex: 0,
			want:       1,
		},
		{
			name: "variable used multiple times — last index wins",
			stmts: []dst.Stmt{
				stmt("ignored"),
				stmt("result"),
				stmt("result"),
			},
			varName:    "result",
			startIndex: 0,
			want:       2,
		},
		{
			name: "variable never used after start",
			stmts: []dst.Stmt{
				stmt("ignored"),
				stmt("other"),
			},
			varName:    "row",
			startIndex: 0,
			want:       0,
		},
		{
			name: "empty varName returns startIndex",
			stmts: []dst.Stmt{
				stmt("anything"),
				stmt("else"),
			},
			varName:    "",
			startIndex: 0,
			want:       0,
		},
		{
			name: "blank identifier returns startIndex",
			stmts: []dst.Stmt{
				stmt("_"), // even if `_` literally appears, we don't follow it
				stmt("_"),
			},
			varName:    "_",
			startIndex: 0,
			want:       0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FindLastUsageOfExecutionResult(tt.stmts, tt.varName, tt.startIndex)
			assert.Equal(t, tt.want, got)
		})
	}
}
