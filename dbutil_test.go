package migration

import (
	"testing"
	"time"
)

func TestTimeVal(t *testing.T) {
	tests := []struct {
		src  interface{}
		want string
	}{
		{
			src:  nil,
			want: "1970-01-01T00:00:00Z",
		},
		{
			src:  time.Date(2099, 12, 31, 23, 59, 59, 0, time.UTC),
			want: "2099-12-31T23:59:59Z",
		},
		{
			src:  "2099-12-31T23:59:59Z",
			want: "2099-12-31T23:59:59Z",
		},
		{
			src:  "2099-12-31T23:59:59+10:00",
			want: "2099-12-31T23:59:59+10:00",
		},
		{
			src:  "2099-12-31 23:59:59+10:00",
			want: "2099-12-31T23:59:59+10:00",
		},
		{
			src:  "2199-12-31 23:59:59+00:00",
			want: "2199-12-31T23:59:59Z",
		},
		{
			src:  int64(1000000000),
			want: time.Unix(1000000000, 0).UTC().Format(time.RFC3339),
		},
		{
			src:  "INVALID",
			want: "1970-01-01T00:00:00Z",
		},
	}
	for tn, tt := range tests {
		var tv timeVal
		err := tv.Scan(tt.src)
		if err != nil {
			t.Errorf("%d: got=%v, want=nil", tn, err)
			continue
		}
		if got, want := tv.Time.Format(time.RFC3339), tt.want; got != want {
			t.Errorf("%d: got=%v, want=%v", tn, got, want)
		}
	}
}
