package nrlogrus

import (
	"go/token"

	"github.com/dave/dst"
)

const (
	NrlogrusImportPath = "github.com/newrelic/go-agent/v3/integrations/logcontext-v2/nrlogrus"
	LogrusImportPath   = "github.com/sirupsen/logrus"
)

// wrapWithNewFormatter returns nrlogrus.NewFormatter(appVar, formatter) as an
// expression suitable for replacing an existing SetFormatter argument.
func wrapWithNewFormatter(appVar string, formatter dst.Expr) dst.Expr {
	return &dst.CallExpr{
		Fun: &dst.Ident{
			Name: "NewFormatter",
			Path: NrlogrusImportPath,
		},
		Args: []dst.Expr{
			&dst.Ident{Name: appVar},
			formatter,
		},
	}
}

// defaultTextFormatterExpr returns &logrus.TextFormatter{}.
func defaultTextFormatterExpr() dst.Expr {
	return &dst.UnaryExpr{
		Op: token.AND,
		X: &dst.CompositeLit{
			Type: &dst.Ident{
				Name: "TextFormatter",
				Path: LogrusImportPath,
			},
		},
	}
}

// defaultSetFormatterStmt builds `<fun>(nrlogrus.NewFormatter(appVar, &logrus.TextFormatter{}))`.
// Callers construct fun to select either a logger-receiver call
// (`logger.SetFormatter(...)`) or the package-level call (`logrus.SetFormatter(...)`).
func defaultSetFormatterStmt(fun dst.Expr, appVar string) *dst.ExprStmt {
	return &dst.ExprStmt{
		X: &dst.CallExpr{
			Fun: fun,
			Args: []dst.Expr{
				wrapWithNewFormatter(appVar, defaultTextFormatterExpr()),
			},
		},
		Decs: dst.ExprStmtDecorations{
			NodeDecs: dst.NodeDecs{Before: dst.NewLine},
		},
	}
}

// loggerSetFormatterFun returns the `loggerName.SetFormatter` selector expr.
func loggerSetFormatterFun(loggerName string) dst.Expr {
	return &dst.SelectorExpr{
		X:   &dst.Ident{Name: loggerName},
		Sel: &dst.Ident{Name: "SetFormatter"},
	}
}

// packageSetFormatterFun returns the `logrus.SetFormatter` ident (path-qualified).
func packageSetFormatterFun() dst.Expr {
	return &dst.Ident{
		Name: "SetFormatter",
		Path: LogrusImportPath,
	}
}
