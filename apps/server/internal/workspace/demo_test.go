package workspace

import "testing"

func TestDemoPresets(t *testing.T) {
	tests := []struct {
		size      DemoSize
		wantLinks int
		wantDays  int
	}{
		{DemoStarter, 15, 90},
		{DemoBusy, 500, 365},
		{DemoFiveYear, 2500, 1825},
	}
	for _, tt := range tests {
		links, days, ok := DemoEstimate(tt.size)
		if !ok || links != tt.wantLinks || days != tt.wantDays {
			t.Fatalf("DemoEstimate(%q) = (%d, %d, %v), want (%d, %d, true)", tt.size, links, days, ok, tt.wantLinks, tt.wantDays)
		}
	}
	if _, _, ok := DemoEstimate("huge"); ok {
		t.Fatal("unknown demo size was accepted")
	}
}
