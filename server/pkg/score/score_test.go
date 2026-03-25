package score

import "testing"

func TestFromCO2(t *testing.T) {
	cases := map[float64]string{
		0.05: "A",
		0.20: "B",
		0.30: "C",
		0.75: "D",
		1.50: "E",
		2.50: "F",
	}

	for input, want := range cases {
		if got := FromCO2(input); got != want {
			t.Fatalf("for %.2f expected %s, got %s", input, want, got)
		}
	}
}

