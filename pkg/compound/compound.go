package compound

import (
	"math"
	"strconv"
	"strings"
)

type Compounder interface {
	Volume(volume float64) (float64, error)
}

func round(num float64) int {
	return int(num + math.Copysign(0.5, num))
}

func toFixed(num float64, precision int) float64 {
	output := math.Pow(10, float64(precision))
	return float64(round(num*output)) / output
}

// floatPrecision returns the number of digits after .
func floatPrecision(f float64) int {
	strVal := strconv.FormatFloat(f, 'f', -1, 64)
	split := strings.Split(strVal, ".")

	// digits after floating point
	daf := 0
	if len(split) > 1 {
		daf = len(split[1])
	}

	return daf
}
