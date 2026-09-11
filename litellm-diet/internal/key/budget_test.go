package key

import (
	"testing"
	"time"
)

func TestApplyResetExpiredOngoingAndNoDuration(t *testing.T) {
	now := time.Date(2026, 9, 11, 12, 0, 0, 0, time.UTC)
	past := now.Add(-time.Minute)
	future := now.Add(time.Hour)
	budget := 5.0

	t.Run("expired window", func(t *testing.T) {
		k := ApplyReset(Key{
			Spend:          4,
			MaxBudget:      &budget,
			BudgetDuration: "1h",
			BudgetResetAt:  &past,
		}, now)
		if k.Spend != 0 {
			t.Errorf("spend = %g, want 0", k.Spend)
		}
		if k.BudgetResetAt == nil || !k.BudgetResetAt.After(now) {
			t.Errorf("reset = %v, want after %v", k.BudgetResetAt, now)
		}
	})

	t.Run("ongoing window", func(t *testing.T) {
		k := ApplyReset(Key{
			Spend:          4,
			BudgetDuration: "1h",
			BudgetResetAt:  &future,
		}, now)
		if k.Spend != 4 {
			t.Errorf("spend = %g, want 4", k.Spend)
		}
		if !k.BudgetResetAt.Equal(future) {
			t.Errorf("reset changed to %v", k.BudgetResetAt)
		}
	})

	t.Run("key without duration", func(t *testing.T) {
		k := ApplyReset(Key{Spend: 9, BudgetResetAt: &past}, now)
		if k.Spend != 9 {
			t.Errorf("spend = %g, want 9", k.Spend)
		}
	})
}

func TestParseDuration(t *testing.T) {
	cases := []struct {
		name    string
		in      string
		want    time.Duration
		wantErr bool
	}{
		{name: "hours", in: "1h", want: time.Hour},
		{name: "days", in: "2d", want: 48 * time.Hour},
		{name: "week", in: "1w", want: 7 * 24 * time.Hour},
		{name: "month", in: "1mo", want: 30 * 24 * time.Hour},
		{name: "empty", in: "", wantErr: true},
		{name: "unknown unit", in: "1y", wantErr: true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := ParseDuration(c.in)
			if c.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != c.want {
				t.Errorf("duration = %s, want %s", got, c.want)
			}
		})
	}
}
