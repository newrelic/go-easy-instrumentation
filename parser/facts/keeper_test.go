package facts

import (
	"testing"
)

func TestKeeper_AddFact(t *testing.T) {
	type args struct {
		entry Entry
	}
	tests := []struct {
		name    string
		fm      Keeper
		args    args
		wantErr bool
	}{
		{
			name: "add valid fact",
			fm:   NewKeeper(),
			args: args{
				entry: Entry{
					Name: "test",
					Fact: GrpcServerType,
				},
			},
			wantErr: false,
		},
		{
			name: "add unknown fact",
			fm:   NewKeeper(),
			args: args{
				entry: Entry{
					Name: "test",
					Fact: 20,
				},
			},
			wantErr: true,
		},
		{
			name: "add None fact",
			fm:   NewKeeper(),
			args: args{
				entry: Entry{
					Name: "test",
					Fact: None,
				},
			},
			wantErr: true,
		},
		{
			name: "add empty name",
			fm:   NewKeeper(),
			args: args{
				entry: Entry{
					Name: "",
					Fact: GrpcServerStream,
				},
			},
			wantErr: true,
		},
		{
			name: "add duplicate entry",
			fm:   Keeper{"test": GrpcServerType},
			args: args{
				entry: Entry{
					Name: "test",
					Fact: GrpcServerType,
				},
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.fm.AddFact(tt.args.entry)
			if err != nil {
				if !tt.wantErr {
					t.Errorf("Keeper.AddFact() error = %v, wantErr %v", err, tt.wantErr)
				}
			} else {
				fact, ok := tt.fm[tt.args.entry.Name]
				if !ok {
					t.Errorf("fact %s not found in the fact manager", tt.args.entry.Name)
				}
				if fact != tt.args.entry.Fact {
					t.Errorf("fact %s is type %s; wanted type %s", tt.args.entry.Name, fact, tt.args.entry.Fact)
				}
			}
		})
	}
}

func TestKeeper_GetFact(t *testing.T) {
	type args struct {
		name string
	}
	tests := []struct {
		name string
		fm   Keeper
		args args
		want Fact
	}{
		{
			name: "get existing fact",
			fm:   Keeper{"test": GrpcServerType},
			args: args{
				name: "test",
			},
			want: GrpcServerType,
		},
		{
			name: "get unknown fact",
			fm:   Keeper{"test": GrpcServerType},
			args: args{
				name: "unknown",
			},
			want: None,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.fm.GetFact(tt.args.name); got != tt.want {
				t.Errorf("Keeper.GetFact() = %v, want %v", got, tt.want)
			}
		})
	}
}
