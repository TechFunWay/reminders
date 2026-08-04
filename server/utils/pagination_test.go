package utils

import "testing"

func TestNormalizePage(t *testing.T) {
	cases := []struct {
		page, size         int
		wantPage, wantSize int
	}{
		{0, 0, 1, DefaultPageSize},
		{-5, -5, 1, DefaultPageSize},
		{2, 50, 2, 50},
		{3, 1000, 3, MaxPageSize},
	}
	for _, c := range cases {
		p, s := NormalizePage(c.page, c.size)
		if p != c.wantPage || s != c.wantSize {
			t.Errorf("NormalizePage(%d,%d) = (%d,%d), want (%d,%d)", c.page, c.size, p, s, c.wantPage, c.wantSize)
		}
	}
}

func TestOffset(t *testing.T) {
	if got := Offset(1, 20); got != 0 {
		t.Errorf("Offset(1,20) = %d, want 0", got)
	}
	if got := Offset(3, 20); got != 40 {
		t.Errorf("Offset(3,20) = %d, want 40", got)
	}
}

func TestAtoi(t *testing.T) {
	if got := Atoi("", 7); got != 7 {
		t.Errorf("Atoi empty = %d, want 7", got)
	}
	if got := Atoi("abc", 7); got != 7 {
		t.Errorf("Atoi invalid = %d, want 7", got)
	}
	if got := Atoi("42", 7); got != 42 {
		t.Errorf("Atoi 42 = %d, want 42", got)
	}
}
