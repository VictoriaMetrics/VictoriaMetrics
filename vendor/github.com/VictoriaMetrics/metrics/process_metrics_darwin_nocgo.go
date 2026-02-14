//go:build darwin && !ios && !cgo

package metrics

func getMemory() (*memoryInfo, error) {
	return nil, errNotImplemented
}
