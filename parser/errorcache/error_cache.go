package errorcache

import (
	"github.com/dave/dst"
)

type ErrorCache struct {
	errorexpr dst.Expr
	errorstmt dst.Stmt
}

func (ec *ErrorCache) Load(errorexpr dst.Expr, errorstmt dst.Stmt) {
	ec.errorexpr = errorexpr
	ec.errorstmt = errorstmt

}

func (ec *ErrorCache) GetExpression() dst.Expr {
	return ec.errorexpr
}
func (ec *ErrorCache) GetStatement() dst.Stmt {
	return ec.errorstmt
}

func (ec *ErrorCache) Clear() {
	ec.errorexpr = nil
	ec.errorstmt = nil
}
