package parser

import (
	"testing"

	"github.com/dave/dst"
)

func Test_segmentOpts_name(t *testing.T) {
	type fields struct {
		async  bool
		create bool
	}
	type args struct {
		fn *dst.FuncDecl
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   string
	}{
		{
			name: "Test_segmentOpts_name",
			fields: fields{
				async:  false,
				create: true,
			},
			args: args{
				fn: &dst.FuncDecl{Name: &dst.Ident{Name: "foo"}},
			},
			want: "foo",
		},
		{
			name: "Test_segmentOpts_name",
			fields: fields{
				async:  true,
				create: true,
			},
			args: args{
				fn: &dst.FuncDecl{Name: &dst.Ident{Name: "foo"}},
			},
			want: "async foo",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opt := &segmentOpts{
				async:  tt.fields.async,
				create: tt.fields.create,
			}
			if got := opt.name(tt.args.fn); got != tt.want {
				t.Errorf("segmentOpts.name() = %v, want %v", got, tt.want)
			}
		})
	}
}
