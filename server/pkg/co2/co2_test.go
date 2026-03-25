package co2

import "testing"

func TestFromBytes(t *testing.T) {
	got := FromBytes(923144)
	want := 0.2448
	if got != want {
		t.Fatalf("expected %.4f, got %.4f", want, got)
	}
}

