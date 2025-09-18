package filestream

func (st *streamTracker) adviseDontNeed(_ int, _ bool) error {
	return nil
}

func (st *streamTracker) close() error {
	return nil
}
