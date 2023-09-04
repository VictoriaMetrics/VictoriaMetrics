package formatutil

import (
	"fmt"
	"math"
)

// HumanizeBytes returns human-readable representation of size in bytes with 1024 base.
func HumanizeBytes(size float64) string {
	prefix := ""
	for _, p := range []string{"ki", "Mi", "Gi", "Ti", "Pi", "Ei", "Zi", "Yi"} {
		if math.Abs(size) < 1024 {
			break
		}
		prefix = p
		size /= 1024
	}
	return fmt.Sprintf("%.4g%s", size, prefix)
}
