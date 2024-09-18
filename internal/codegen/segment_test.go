package codegen

import (
	"go/token"
	"reflect"
	"testing"

	"github.com/dave/dst"
	"github.com/stretchr/testify/assert"
)

func Test_endExternalSegment(t *testing.T) {
	type args struct {
		segmentName string
		nodeDecs    *dst.NodeDecs
	}
	tests := []struct {
		name string
		args args
		want *dst.ExprStmt
	}{
		{
			name: "end_external_segment",
			args: args{
				segmentName: "example",
				nodeDecs: &dst.NodeDecs{
					After: dst.NewLine,
					End:   []string{"// this is a comment", "// this is also a comment"},
				},
			},
			want: &dst.ExprStmt{
				X: &dst.CallExpr{
					Fun: &dst.SelectorExpr{
						X:   dst.NewIdent("example"),
						Sel: dst.NewIdent("End"),
					},
				},
				Decs: dst.ExprStmtDecorations{
					NodeDecs: dst.NodeDecs{
						After: dst.NewLine,
						End:   []string{"// this is a comment", "// this is also a comment"},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := EndExternalSegment(tt.args.segmentName, tt.args.nodeDecs); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("endExternalSegment() = %v, want %v", got, tt.want)
			}
			if len(tt.args.nodeDecs.End) != 0 {
				t.Errorf("endExternalSegment() should clear the End decorations slice but did NOT")
			}
			if tt.args.nodeDecs.After != dst.None {
				t.Errorf("endExternalSegment() should set the After decorations slice to \"None\" but it was %s", tt.args.nodeDecs.After.String())
			}
		})
	}
}

func Test_startExternalSegment(t *testing.T) {
	type args struct {
		request    dst.Expr
		txnVar     string
		segmentVar string
		nodeDecs   *dst.NodeDecs
	}
	tests := []struct {
		name string
		args args
		want *dst.AssignStmt
	}{
		{
			name: "start_external_segment",
			args: args{
				request:    &dst.Ident{Name: "r", Path: HttpImportPath},
				txnVar:     "txn",
				segmentVar: "example",
				nodeDecs: &dst.NodeDecs{
					Before: dst.NewLine,
					Start:  []string{"// this is a comment"},
				},
			},
			want: &dst.AssignStmt{
				Tok: token.DEFINE,
				Lhs: []dst.Expr{
					dst.NewIdent("example"),
				},
				Rhs: []dst.Expr{
					&dst.CallExpr{
						Fun: &dst.Ident{
							Name: "StartExternalSegment",
							Path: NewRelicAgentImportPath,
						},
						Args: []dst.Expr{
							dst.NewIdent("txn"),
							dst.Clone(&dst.Ident{Name: "r", Path: HttpImportPath}).(dst.Expr),
						},
					},
				},
				Decs: dst.AssignStmtDecorations{
					NodeDecs: dst.NodeDecs{
						Before: dst.NewLine,
						Start:  []string{"// this is a comment"},
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := StartExternalSegment(tt.args.request, tt.args.txnVar, tt.args.segmentVar, tt.args.nodeDecs); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("startExternalSegment() = %v, want %v", got, tt.want)
			}
			if len(tt.args.nodeDecs.Start) != 0 {
				t.Errorf("should clear the End decorations slice but did NOT")
			}
			if tt.args.nodeDecs.Before != dst.None {
				t.Errorf("should set the Before decorations slice to \"None\" but it was %s", tt.args.nodeDecs.Before.String())
			}
		})
	}
}

func Test_captureHttpResponse(t *testing.T) {
	type args struct {
		segmentVariable  string
		responseVariable dst.Expr
	}
	tests := []struct {
		name string
		args args
		want *dst.AssignStmt
	}{
		{
			name: "capture_http_response",
			args: args{
				segmentVariable: "example",
				responseVariable: &dst.Ident{
					Name: "resp",
					Path: HttpImportPath,
				},
			},
			want: &dst.AssignStmt{
				Lhs: []dst.Expr{
					&dst.SelectorExpr{
						X:   dst.NewIdent("example"),
						Sel: dst.NewIdent("Response"),
					},
				},
				Rhs: []dst.Expr{
					dst.Clone(&dst.Ident{
						Name: "resp",
						Path: HttpImportPath,
					}).(dst.Expr),
				},
				Tok: token.ASSIGN,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := CaptureHttpResponse(tt.args.segmentVariable, tt.args.responseVariable); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("captureHttpResponse() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_deferSegment(t *testing.T) {
	type args struct {
		segmentName string
		txnVarName  string
	}
	tests := []struct {
		name string
		args args
		want *dst.DeferStmt
	}{
		{
			name: "Test defer segment",
			args: args{
				segmentName: "testSegment",
				txnVarName:  "testTxn",
			},
			want: &dst.DeferStmt{
				Call: &dst.CallExpr{
					Fun: &dst.SelectorExpr{
						X: &dst.CallExpr{
							Fun: &dst.SelectorExpr{
								X: dst.NewIdent("testTxn"),
								Sel: &dst.Ident{
									Name: "StartSegment",
								},
							},
							Args: []dst.Expr{
								&dst.BasicLit{
									Kind:  token.STRING,
									Value: `"testSegment"`,
								},
							},
						},
						Sel: &dst.Ident{
							Name: "End",
						},
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DeferSegment(tt.args.segmentName, tt.args.txnVarName)
			assert.Equal(t, tt.want, got)
		})
	}
}
