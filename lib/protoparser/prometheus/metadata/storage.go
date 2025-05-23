package metadata

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/prometheus"
)

// Storage manages metadata collection and persistence
type Storage struct {
	mu       sync.RWMutex
	metadata map[string]*prometheus.MetricMetadata
	filePath string
	
	// Configuration
	flushInterval time.Duration
	enabled       bool
	
	// Control
	stopCh chan struct{}
	wg     sync.WaitGroup
}

// PrometheusMetadataResponse represents the response format from Prometheus /api/v1/metadata endpoint
type PrometheusMetadataResponse struct {
	Status string                                         `json:"status"`
	Data   map[string][]prometheus.MetricMetadata        `json:"data"`
}

// NewStorage creates a new metadata storage instance
func NewStorage(filePath string, flushInterval time.Duration) *Storage {
	if filePath == "" {
		filePath = "vmagent_metadata.json"
	}
	if flushInterval <= 0 {
		flushInterval = 30 * time.Second
	}
	
	return &Storage{
		metadata:      make(map[string]*prometheus.MetricMetadata),
		filePath:      filePath,
		flushInterval: flushInterval,
		enabled:       true,
		stopCh:        make(chan struct{}),
	}
}

// Start begins the periodic flushing of metadata to disk
func (s *Storage) Start() {
	s.wg.Add(1)
	go s.flushLoop()
}

// Stop stops the metadata storage and performs a final flush
func (s *Storage) Stop() {
	close(s.stopCh)
	s.wg.Wait()
	s.Flush() // Final flush before stopping
}

// Callback returns a MetadataCallback function for use with prometheus parsers
func (s *Storage) Callback() prometheus.MetadataCallback {
	if !s.enabled {
		return nil
	}
	return s.handleMetadata
}

// handleMetadata processes metadata entries from the parser
func (s *Storage) handleMetadata(metadata *prometheus.MetricMetadata) {
	if metadata == nil || metadata.MetricFamilyName == "" {
		return
	}
	
	s.mu.Lock()
	defer s.mu.Unlock()
	
	existing, ok := s.metadata[metadata.MetricFamilyName]
	if !ok {
		// Create new entry
		s.metadata[metadata.MetricFamilyName] = &prometheus.MetricMetadata{
			MetricFamilyName: metadata.MetricFamilyName,
			Type:             metadata.Type,
			Help:             metadata.Help,
			Unit:             metadata.Unit,
		}
	} else {
		// Update existing entry
		if metadata.Type != "" {
			existing.Type = metadata.Type
		}
		if metadata.Help != "" {
			existing.Help = metadata.Help
		}
		if metadata.Unit != "" {
			existing.Unit = metadata.Unit
		}
	}
}

// flushLoop runs the periodic flush operation
func (s *Storage) flushLoop() {
	defer s.wg.Done()
	
	ticker := time.NewTicker(s.flushInterval)
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			s.Flush()
		case <-s.stopCh:
			return
		}
	}
}

// Flush writes metadata to disk in Prometheus API format
func (s *Storage) Flush() {
	s.mu.RLock()
	
	if len(s.metadata) == 0 {
		s.mu.RUnlock()
		return
	}
	
	// Convert to Prometheus API format
	data := make(map[string][]prometheus.MetricMetadata)
	for name, metadata := range s.metadata {
		data[name] = []prometheus.MetricMetadata{*metadata}
	}
	
	response := PrometheusMetadataResponse{
		Status: "success",
		Data:   data,
	}
	s.mu.RUnlock()
	
	// Write to file
	if err := s.writeToFile(&response); err != nil {
		logger.Errorf("failed to write metadata to file %q: %s", s.filePath, err)
	}
}

// writeToFile writes the metadata response to the configured file
func (s *Storage) writeToFile(response *PrometheusMetadataResponse) error {
	// Ensure directory exists
	dir := filepath.Dir(s.filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("cannot create directory %q: %w", dir, err)
	}
	
	// Write to temporary file first
	tempFile := s.filePath + ".tmp"
	file, err := os.Create(tempFile)
	if err != nil {
		return fmt.Errorf("cannot create temp file %q: %w", tempFile, err)
	}
	defer file.Close()
	
	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(response); err != nil {
		return fmt.Errorf("cannot encode metadata: %w", err)
	}
	
	if err := file.Sync(); err != nil {
		return fmt.Errorf("cannot sync temp file: %w", err)
	}
	
	file.Close()
	
	// Atomic move
	if err := os.Rename(tempFile, s.filePath); err != nil {
		return fmt.Errorf("cannot move temp file to final location: %w", err)
	}
	
	return nil
}

// GetStats returns statistics about the metadata storage
func (s *Storage) GetStats() (int, string) {
	s.mu.RLock()
	count := len(s.metadata)
	s.mu.RUnlock()
	
	return count, s.filePath
} 