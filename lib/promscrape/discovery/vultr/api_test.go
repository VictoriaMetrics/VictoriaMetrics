package vultr

import (
	"testing"
)

func TestNewAPIConfig(t *testing.T) {

	sdc := &SDConfig{}
	baseDir := "."
	_, err := newAPIConfig(sdc, baseDir)
	if err != nil {
		t.Errorf("newAPIConfig failed with, err: %v", err)
		return
	}
}
