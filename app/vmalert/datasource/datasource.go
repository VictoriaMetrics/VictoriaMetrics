package datasource

import "context"

// Metrics the data returns from storage
type Metrics struct{}

// VMStorage represents vmstorage entity with ability to read and write metrics
type VMStorage struct{}

//Query basic query to the datasource
func (s *VMStorage) Query(ctx context.Context, query string) ([]Metrics, error) {
	return nil, nil
}
