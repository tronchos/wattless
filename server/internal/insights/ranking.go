package insights

// SeverityRank maps a severity label to a numeric rank for sorting.
func SeverityRank(value string) int {
	switch value {
	case "high":
		return 3
	case "medium":
		return 2
	default:
		return 1
	}
}

// ConfidenceRank maps a confidence label to a numeric rank for sorting.
func ConfidenceRank(value string) int {
	switch value {
	case "high":
		return 3
	case "medium":
		return 2
	default:
		return 1
	}
}
