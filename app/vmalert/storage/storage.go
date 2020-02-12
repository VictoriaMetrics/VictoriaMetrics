package storage

// Metrics the data returns from storage
type Metrics struct{}

// VMStorage represents vmstorage entity with ability to read and write metrics
type VMStorage struct{}

func (s *VMStorage) ReadMetrics(query string) ([]Metrics, error) {
	return nil, nil
}
