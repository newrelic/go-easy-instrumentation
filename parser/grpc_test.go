package parser

import (
	"go/token"
	"reflect"
	"testing"

	"github.com/dave/dst"
	"github.com/newrelic/go-easy-instrumentation/parser/codegen"
	"github.com/stretchr/testify/assert"
)

func TestInstrumentGrpcDial(t *testing.T) {
	tests := []struct {
		name   string
		code   string
		expect string
	}{
		{
			name: "detect and trace grpc dial",
			code: `package main

import "google.golang.org/grpc"

func main() {
	conn, err := grpc.Dial(
		"localhost:8080",
		grpc.WithInsecure(),
	)
	if err != nil {
		panic(err)
	}
	defer conn.Close()
}
`,
			expect: `package main

import (
	"github.com/newrelic/go-agent/v3/integrations/nrgrpc"
	"google.golang.org/grpc"
)

func main() {
	conn, err := grpc.Dial(
		"localhost:8080",
		grpc.WithInsecure(),
		grpc.WithUnaryInterceptor(nrgrpc.UnaryClientInterceptor),
		grpc.WithStreamInterceptor(nrgrpc.StreamClientInterceptor),
	)
	if err != nil {
		panic(err)
	}
	defer conn.Close()
}
`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer panicRecovery(t)
			got := testStatelessTracingFunction(t, tt.code, InstrumentGrpcDial)
			assert.Equal(t, tt.expect, got)
		})
	}
}

func TestInstrumentGrpcServer(t *testing.T) {
	tests := []struct {
		name   string
		code   string
		expect string
	}{
		{
			name: "detect and trace grpc dial",
			code: `package main

import "google.golang.org/grpc"

func main() {
	lis, err := net.Listen("tcp", "localhost:8080")
	grpcServer := grpc.NewServer()
	grpcServer.Serve(lis)
}
`,
			expect: `package main

import (
	"github.com/newrelic/go-agent/v3/integrations/nrgrpc"
	"google.golang.org/grpc"
)

func main() {
	lis, err := net.Listen("tcp", "localhost:8080")
	grpcServer := grpc.NewServer(
		grpc.UnaryInterceptor(nrgrpc.UnaryServerInterceptor(app)),
		grpc.StreamInterceptor(nrgrpc.StreamServerInterceptor(app)),
	)
	grpcServer.Serve(lis)
}
`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer panicRecovery(t)
			got := testStatefulTracingFunction(t, tt.code, InstrumentGrpcServer, false)
			assert.Equal(t, tt.expect, got)
		})
	}
}

func Test_grpcDialCall(t *testing.T) {
	type args struct {
		node dst.Node
	}
	tests := []struct {
		name  string
		args  args
		want  *dst.CallExpr
		want1 bool
	}{
		{
			name: "grpc Dial Assign Statement",
			args: args{
				node: &dst.AssignStmt{
					Rhs: []dst.Expr{
						&dst.CallExpr{
							Fun: &dst.Ident{
								Name: "Dial",
								Path: codegen.GrpcImportPath,
							},
							Args: []dst.Expr{
								&dst.BasicLit{
									Value: `"localhost:8080"`,
									Kind:  token.STRING,
								},
							},
						},
					},
					Lhs: []dst.Expr{
						&dst.Ident{
							Name: "conn",
						},
						&dst.Ident{
							Name: "err",
						},
					},
				},
			},
			want: &dst.CallExpr{
				Fun: &dst.Ident{
					Name: "Dial",
					Path: codegen.GrpcImportPath,
				},
				Args: []dst.Expr{
					&dst.BasicLit{
						Value: `"localhost:8080"`,
						Kind:  token.STRING,
					},
				},
			},
			want1: true,
		},
		{
			name: "grpc Dial Expression Statement",
			args: args{
				node: &dst.ExprStmt{
					X: &dst.CallExpr{
						Fun: &dst.Ident{
							Name: "Dial",
							Path: codegen.GrpcImportPath,
						},
						Args: []dst.Expr{
							&dst.BasicLit{
								Value: `"localhost:8080"`,
								Kind:  token.STRING,
							},
						},
					},
				},
			},
			want: &dst.CallExpr{
				Fun: &dst.Ident{
					Name: "Dial",
					Path: codegen.GrpcImportPath,
				},
				Args: []dst.Expr{
					&dst.BasicLit{
						Value: `"localhost:8080"`,
						Kind:  token.STRING,
					},
				},
			},
			want1: true,
		},
		{
			name: "non grpc dial expression",
			args: args{
				node: &dst.ExprStmt{
					X: &dst.CallExpr{
						Fun: &dst.Ident{
							Name: "Dial",
							Path: "github.com/confluentinc/confluent-kafka-go",
						},
						Args: []dst.Expr{
							&dst.BasicLit{
								Value: `"localhost:8080"`,
								Kind:  token.STRING,
							},
						},
					},
				},
			},
			want:  nil,
			want1: false,
		},
		{
			name: "non grpc dial assignment",
			args: args{
				node: &dst.AssignStmt{
					Rhs: []dst.Expr{
						&dst.CallExpr{
							Fun: &dst.Ident{
								Name: "Dial",
								Path: "github.com/confluentinc/confluent-kafka-go",
							},
							Args: []dst.Expr{
								&dst.BasicLit{
									Value: `"localhost:8080"`,
									Kind:  token.STRING,
								},
							},
						},
					},
					Lhs: []dst.Expr{
						&dst.Ident{
							Name: "conn",
						},
						&dst.Ident{
							Name: "err",
						},
					},
				},
			},
			want:  nil,
			want1: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, got1 := grpcDialCall(tt.args.node)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("grpcDialCall() got = %v, want %v", got, tt.want)
			}
			if got1 != tt.want1 {
				t.Errorf("grpcDialCall() got1 = %v, want %v", got1, tt.want1)
			}
		})
	}
}

func Test_grpcNewServerCall(t *testing.T) {
	type args struct {
		node dst.Node
	}
	tests := []struct {
		name  string
		args  args
		want  *dst.CallExpr
		want1 bool
	}{
		{
			name: "grpc NewServer Assign Statement",
			args: args{
				node: &dst.AssignStmt{
					Rhs: []dst.Expr{
						&dst.CallExpr{
							Fun: &dst.Ident{
								Name: "NewServer",
								Path: codegen.GrpcImportPath,
							},
							Args: []dst.Expr{},
						},
					},
					Lhs: []dst.Expr{
						dst.NewIdent("grpcServer"),
					},
				},
			},
			want: &dst.CallExpr{
				Fun: &dst.Ident{
					Name: "NewServer",
					Path: codegen.GrpcImportPath,
				},
				Args: []dst.Expr{},
			},
			want1: true,
		},
		{
			name: "grpc NewServer Expression Statement",
			args: args{
				node: &dst.ExprStmt{
					X: &dst.CallExpr{
						Fun: &dst.Ident{
							Name: "NewServer",
							Path: codegen.GrpcImportPath,
						},
						Args: []dst.Expr{},
					},
				},
			},
			want: &dst.CallExpr{
				Fun: &dst.Ident{
					Name: "NewServer",
					Path: codegen.GrpcImportPath,
				},
				Args: []dst.Expr{},
			},
			want1: true,
		},
		{
			name: "non grpc Assign Statement",
			args: args{
				node: &dst.AssignStmt{
					Rhs: []dst.Expr{
						&dst.CallExpr{
							Fun: &dst.Ident{
								Name: "NewServer",
								Path: "github.com/confluentinc/confluent-kafka-go",
							},
							Args: []dst.Expr{},
						},
					},
					Lhs: []dst.Expr{
						dst.NewIdent("grpcServer"),
					},
				},
			},
			want:  nil,
			want1: false,
		},
		{
			name: "grpc NewServer Expression Statement",
			args: args{
				node: &dst.ExprStmt{
					X: &dst.CallExpr{
						Fun: &dst.Ident{
							Name: "NewServer",
							Path: "github.com/confluentinc/confluent-kafka-go",
						},
						Args: []dst.Expr{},
					},
				},
			},
			want:  nil,
			want1: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, got1 := grpcNewServerCall(tt.args.node)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("grpcNewServerCall() got = %v, want %v", got, tt.want)
			}
			if got1 != tt.want1 {
				t.Errorf("grpcNewServerCall() got1 = %v, want %v", got1, tt.want1)
			}
		})
	}
}
