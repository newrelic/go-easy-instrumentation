package util

import (
	"go/token"
	"testing"

	"github.com/dave/dst"
	"github.com/stretchr/testify/assert"
)

func TestCompareExpr(t *testing.T) {
	tests := []struct {
		name string
		a    dst.Expr
		b    dst.Expr
		want bool
	}{
		{
			name: "identical basic literals",
			a: &dst.BasicLit{
				Kind:  token.INT,
				Value: "42",
			},
			b: &dst.BasicLit{
				Kind:  token.INT,
				Value: "42",
			},
			want: true,
		},
		{
			name: "different basic literals",
			a: &dst.BasicLit{
				Kind:  token.INT,
				Value: "42",
			},
			b: &dst.BasicLit{
				Kind:  token.INT,
				Value: "43",
			},
			want: false,
		},
		{
			name: "identical identifiers",
			a: &dst.Ident{
				Name: "x",
			},
			b: &dst.Ident{
				Name: "x",
			},
			want: true,
		},
		{
			name: "different identifiers",
			a: &dst.Ident{
				Name: "x",
			},
			b: &dst.Ident{
				Name: "y",
			},
			want: false,
		},
		{
			name: "identical binary expressions",
			a: &dst.BinaryExpr{
				X:  &dst.BasicLit{Kind: token.INT, Value: "1"},
				Op: token.ADD,
				Y:  &dst.BasicLit{Kind: token.INT, Value: "2"},
			},
			b: &dst.BinaryExpr{
				X:  &dst.BasicLit{Kind: token.INT, Value: "1"},
				Op: token.ADD,
				Y:  &dst.BasicLit{Kind: token.INT, Value: "2"},
			},
			want: true,
		},
		{
			name: "different binary expressions",
			a: &dst.BinaryExpr{
				X:  &dst.BasicLit{Kind: token.INT, Value: "1"},
				Op: token.ADD,
				Y:  &dst.BasicLit{Kind: token.INT, Value: "2"},
			},
			b: &dst.BinaryExpr{
				X:  &dst.BasicLit{Kind: token.INT, Value: "1"},
				Op: token.SUB,
				Y:  &dst.BasicLit{Kind: token.INT, Value: "2"},
			},
			want: false,
		},
		{
			name: "identical call expressions",
			a: &dst.CallExpr{
				Fun: &dst.Ident{Name: "foo"},
				Args: []dst.Expr{
					&dst.BasicLit{Kind: token.INT, Value: "1"},
					&dst.BasicLit{Kind: token.INT, Value: "2"},
				},
			},
			b: &dst.CallExpr{
				Fun: &dst.Ident{Name: "foo"},
				Args: []dst.Expr{
					&dst.BasicLit{Kind: token.INT, Value: "1"},
					&dst.BasicLit{Kind: token.INT, Value: "2"},
				},
			},
			want: true,
		},
		{
			name: "different call expressions",
			a: &dst.CallExpr{
				Fun: &dst.Ident{Name: "foo"},
				Args: []dst.Expr{
					&dst.BasicLit{Kind: token.INT, Value: "1"},
					&dst.BasicLit{Kind: token.INT, Value: "2"},
				},
			},
			b: &dst.CallExpr{
				Fun: &dst.Ident{Name: "bar"},
				Args: []dst.Expr{
					&dst.BasicLit{Kind: token.INT, Value: "1"},
					&dst.BasicLit{Kind: token.INT, Value: "2"},
				},
			},
			want: false,
		},
		{
			name: "identical selector expressions",
			a: &dst.SelectorExpr{
				X:   &dst.Ident{Name: "pkg"},
				Sel: &dst.Ident{Name: "Func"},
			},
			b: &dst.SelectorExpr{
				X:   &dst.Ident{Name: "pkg"},
				Sel: &dst.Ident{Name: "Func"},
			},
			want: true,
		},
		{
			name: "different selector expressions",
			a: &dst.SelectorExpr{
				X:   &dst.Ident{Name: "pkg"},
				Sel: &dst.Ident{Name: "Func"},
			},
			b: &dst.SelectorExpr{
				X:   &dst.Ident{Name: "pkg"},
				Sel: &dst.Ident{Name: "Method"},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := AssertExpressionEqual(tt.a, tt.b)
			assert.Equal(t, tt.want, got)
		})
	}
}

func BenchmarkAssertExpressionEqual(b *testing.B) {
	expr1 := &dst.CallExpr{
		Fun: &dst.SelectorExpr{
			X:   &dst.Ident{Name: "pkg"},
			Sel: &dst.Ident{Name: "Func"},
		},
		Args: []dst.Expr{
			&dst.BinaryExpr{
				X:  &dst.BasicLit{Kind: token.INT, Value: "1"},
				Op: token.ADD,
				Y:  &dst.BasicLit{Kind: token.INT, Value: "2"},
			},
			&dst.CallExpr{
				Fun: &dst.Ident{Name: "nestedFunc"},
				Args: []dst.Expr{
					&dst.BasicLit{Kind: token.STRING, Value: `"arg"`},
				},
			},
		},
	}

	expr2 := &dst.CallExpr{
		Fun: &dst.SelectorExpr{
			X:   &dst.Ident{Name: "pkg"},
			Sel: &dst.Ident{Name: "Func"},
		},
		Args: []dst.Expr{
			&dst.BinaryExpr{
				X:  &dst.BasicLit{Kind: token.INT, Value: "1"},
				Op: token.ADD,
				Y:  &dst.BasicLit{Kind: token.INT, Value: "2"},
			},
			&dst.CallExpr{
				Fun: &dst.Ident{Name: "nestedFunc"},
				Args: []dst.Expr{
					&dst.BasicLit{Kind: token.STRING, Value: `"arg"`},
				},
			},
		},
	}

	for i := 0; i < b.N; i++ {
		AssertExpressionEqual(expr1, expr2)
	}
}
