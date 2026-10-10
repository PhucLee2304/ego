package valuex

import "math"

func RoundFloat(value float64, places int) float64 {
	if places <= 0 {
		return math.Round(value)
	}

	factor := math.Pow10(places)
	return math.Round(value*factor) / factor
}
