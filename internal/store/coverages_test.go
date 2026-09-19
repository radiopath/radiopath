package store

import "testing"

func TestComputeTime(t *testing.T) {
	for ms, want := range map[int32]string{732: "732 ms", 9600: "9.6 s", 95732: "1 min 35 s", 600000: "10 min 0 s"} {
		if got := (Coverage{ComputeMs: ms}).ComputeTime(); got != want {
			t.Errorf("%d ms: got %q, want %q", ms, got, want)
		}
	}
}
