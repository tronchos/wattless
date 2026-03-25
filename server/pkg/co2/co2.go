package co2

import "math"

const (
	bytesPerGigabyte         = 1_000_000_000
	energyPerGigabyteKwh     = 0.8
	returningVisitFactor     = 0.75
	globalGridIntensityGrams = 442
)

func FromBytes(totalBytes int64) float64 {
	grams := (float64(totalBytes) / bytesPerGigabyte) * energyPerGigabyteKwh * returningVisitFactor * globalGridIntensityGrams
	return math.Round(grams*10_000) / 10_000
}

