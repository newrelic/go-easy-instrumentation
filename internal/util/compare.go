package util

import (
	"reflect"

	"github.com/dave/dst"
)

func AssertExpressionEqual(a dst.Expr, b dst.Expr) bool {
	return compareExpr(a, b)
}

func compareExpr(a dst.Expr, b dst.Expr) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	if reflect.TypeOf(a) != reflect.TypeOf(b) {
		return false
	}

	switch a := a.(type) {
	case *dst.BasicLit:
		b := b.(*dst.BasicLit)
		return a.Kind == b.Kind && a.Value == b.Value
	case *dst.Ident:
		b := b.(*dst.Ident)
		return a.Name == b.Name
	case *dst.BinaryExpr:
		b := b.(*dst.BinaryExpr)
		return compareExpr(a.X, b.X) && compareExpr(a.Y, b.Y) && a.Op == b.Op
	case *dst.CallExpr:
		b := b.(*dst.CallExpr)
		if !compareExpr(a.Fun, b.Fun) {
			return false
		}
		if len(a.Args) != len(b.Args) {
			return false
		}
		for i := range a.Args {
			if !compareExpr(a.Args[i], b.Args[i]) {
				return false
			}
		}
		return true
	case *dst.ParenExpr:
		b := b.(*dst.ParenExpr)
		return compareExpr(a.X, b.X)
	case *dst.SelectorExpr:
		b := b.(*dst.SelectorExpr)
		return compareExpr(a.X, b.X) && compareExpr(a.Sel, b.Sel)
	case *dst.StarExpr:
		b := b.(*dst.StarExpr)
		return compareExpr(a.X, b.X)
	case *dst.UnaryExpr:
		b := b.(*dst.UnaryExpr)
		return a.Op == b.Op && compareExpr(a.X, b.X)
	default:
		return false
	}
}
