package analytics

import (
	"testing"
	"time"
)

func TestReadBackoff(t *testing.T) {
	cases := map[int]time.Duration{
		0:   time.Second, // defensive: never zero, which would spin
		1:   time.Second,
		2:   2 * time.Second,
		3:   4 * time.Second,
		4:   8 * time.Second,
		5:   16 * time.Second,
		6:   maxReadBackoff, // 32s would exceed the cap
		7:   maxReadBackoff,
		100: maxReadBackoff, // must not overflow the shift
	}

	for failures, want := range cases {
		if got := readBackoff(failures); got != want {
			t.Errorf("readBackoff(%d) = %v, want %v", failures, got, want)
		}
	}
}

func TestReadBackoffIsMonotonicAndBounded(t *testing.T) {
	var previous time.Duration
	for failures := 1; failures <= 50; failures++ {
		d := readBackoff(failures)
		if d < previous {
			t.Fatalf("readBackoff(%d) = %v, which is shorter than the previous %v", failures, d, previous)
		}
		if d > maxReadBackoff {
			t.Fatalf("readBackoff(%d) = %v, above the %v cap", failures, d, maxReadBackoff)
		}
		if d <= 0 {
			t.Fatalf("readBackoff(%d) = %v; a non-positive delay would spin the loop", failures, d)
		}
		previous = d
	}
}
