package reminder

import (
	"testing"
	"time"
)

func TestNextQQGatewayRetryCapsAtMaximum(t *testing.T) {
	cases := []struct {
		current time.Duration
		want    time.Duration
	}{
		{0, qqGatewayRetryInitial},
		{qqGatewayRetryInitial, time.Minute},
		{time.Minute, 2 * time.Minute},
		{2 * time.Minute, 4 * time.Minute},
		{4 * time.Minute, qqGatewayRetryMax},
		{qqGatewayRetryMax, qqGatewayRetryMax},
	}
	for _, tc := range cases {
		if got := nextQQGatewayRetry(tc.current); got != tc.want {
			t.Errorf("nextQQGatewayRetry(%s) = %s, want %s", tc.current, got, tc.want)
		}
	}
}
