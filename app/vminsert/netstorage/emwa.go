package netstorage

// Copied and pasted from github.com/VividCortex/ewma to avoid adding a new dependency.
// Reduced to only the needed functionality.

const (
	// For best results, the moving average should not be initialized to the
	// samples it sees immediately. The book "Production and Operations
	// Analysis" by Steven Nahmias suggests initializing the moving average to
	// the mean of the first 10 samples. Until the VariableEwma has seen this
	// many samples, it is not "ready" to be queried for the value of the
	// moving average. This adds some memory cost.
	warmupSamples uint8 = 10
)

func newMovingAverage(age float64) *variableEWMA {
	return &variableEWMA{
		decay: 2 / (age + 1),
	}
}

// variableEWMA represents the exponentially weighted moving average of a series of
// numbers. Unlike SimpleEWMA, it supports a custom age, and thus uses more memory.
type variableEWMA struct {
	// The multiplier factor by which the previous samples decay.
	decay float64
	// The current value of the average.
	value float64
	// The number of samples added to this instance.
	count uint8
}

// Add adds a value to the series and updates the moving average.
func (e *variableEWMA) Add(value float64) {
	switch {
	case e.count < warmupSamples:
		e.count++
		e.value += value
	case e.count == warmupSamples:
		e.count++
		e.value = e.value / float64(warmupSamples)
		e.value = (value * e.decay) + (e.value * (1 - e.decay))
	default:
		e.value = (value * e.decay) + (e.value * (1 - e.decay))
	}
}

// Value returns the current value of the average, or 0.0 if the series hasn't
// warmed up yet.
func (e *variableEWMA) Value() float64 {
	if e.count <= warmupSamples {
		return 0.0
	}

	return e.value
}

// Set sets the EWMA's value.
func (e *variableEWMA) Set(value float64) {
	e.value = value
	if e.count <= warmupSamples {
		e.count = warmupSamples + 1
	}
}
