package facts

import "testing"

func TestFact_String(t *testing.T) {
	tests := []struct {
		name string
		f    Fact
		want string
	}{
		{
			name: "None",
			f:    None,
			want: "None",
		},
		{
			name: "GrpcServer",
			f:    GrpcServerType,
			want: "GrpcServer",
		},
		{
			name: "Unkwnown",
			f:    20,
			want: "Unknown",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.f.String(); got != tt.want {
				t.Errorf("Fact.String() = %v, want %v", got, tt.want)
			}
		})
	}
}
