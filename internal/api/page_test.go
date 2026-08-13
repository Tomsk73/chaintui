package api

import "testing"

func TestPageOptsSize(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in, want int32
	}{
		{0, DefaultPageSize},
		{-1, DefaultPageSize},
		{10, 10},
		{50, 50},
		{200, 200},
		{201, 200},
		{1000, 200},
	}
	for _, tc := range cases {
		got := (PageOpts{PageSize: tc.in}).size()
		if got != tc.want {
			t.Errorf("PageSize=%d: got %d, want %d", tc.in, got, tc.want)
		}
	}
}
