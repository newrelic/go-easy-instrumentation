package facts

import "testing"

func TestEntry_String(t *testing.T) {
	type fields struct {
		Name string
		Fact Fact
	}
	tests := []struct {
		name   string
		fields fields
		want   string
	}{
		{
			name: "valid entry",
			fields: fields{
				Name: "test",
				Fact: GrpcServerType,
			},
			want: "{Name: test, Fact: GrpcServer}",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := Entry{
				Name: tt.fields.Name,
				Fact: tt.fields.Fact,
			}
			if got := e.String(); got != tt.want {
				t.Errorf("Entry.String() = %v, want %v", got, tt.want)
			}
		})
	}
}
