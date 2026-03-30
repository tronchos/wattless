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

// FindingPriorityRank nudges some findings ahead of more generic siblings when
// severity and confidence are otherwise tied.
func FindingPriorityRank(value string) int {
	switch value {
	case "dominant_image_overdelivery":
		return 3
	case "repeated_gallery_overdelivery":
		return 2
	default:
		return 0
	}
}
