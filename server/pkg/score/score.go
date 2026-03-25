package score

func FromCO2(grams float64) string {
	switch {
	case grams <= 0.10:
		return "A"
	case grams <= 0.25:
		return "B"
	case grams <= 0.50:
		return "C"
	case grams <= 1.00:
		return "D"
	case grams <= 2.00:
		return "E"
	default:
		return "F"
	}
}

