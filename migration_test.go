package migration

import "testing"

func TestErrors(t *testing.T) {
	tests := []struct {
		err  error
		want string
	}{
		{
			err: &Error{
				Version:     1,
				Description: "description",
			},
			want: "1: description",
		},
		{
			err: &Errors{
				&Error{Version: 1, Description: "xxxx1"},
				&Error{Version: 2, Description: "xxxx2"},
			},
			want: "1: xxxx1\n2: xxxx2",
		},
	}
	for tn, tt := range tests {
		if got, want := tt.err.Error(), tt.want; got != want {
			t.Errorf("%d: got=%v, want=%v", tn, got, want)
		}
	}
}
