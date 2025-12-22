package file

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/bhangun/mandau/pkg/plugin"
)

type FileAuditPlugin struct {
	name       string
	version    string
	logDir     string
	currentLog *os.File
	mu         sync.Mutex
	rotateSize int64
}

func New() *FileAuditPlugin {
	return &FileAuditPlugin{
		name:       "file-audit",
		version:    "1.0.0",
		rotateSize: 100 * 1024 * 1024, // 100MB
	}
}

func (p *FileAuditPlugin) Name() string    { return p.name }
func (p *FileAuditPlugin) Version() string { return p.version }

func (p *FileAuditPlugin) Capabilities() []plugin.Capability {
	return []plugin.Capability{plugin.CapabilityAudit}
}

func (p *FileAuditPlugin) Init(ctx context.Context, config map[string]interface{}) error {
	logDir, ok := config["log_dir"].(string)
	if !ok {
		logDir = "/var/log/mandau"
	}
	p.logDir = logDir

	if err := os.MkdirAll(logDir, 0750); err != nil {
		return fmt.Errorf("create log dir: %w", err)
	}

	return p.openLogFile()
}

func (p *FileAuditPlugin) openLogFile() error {
	filename := filepath.Join(p.logDir, fmt.Sprintf("audit-%s.jsonl",
		time.Now().Format("2006-01-02")))

	f, err := os.OpenFile(filename, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0640)
	if err != nil {
		return err
	}

	p.currentLog = f
	return nil
}

func (p *FileAuditPlugin) Log(ctx context.Context, entry *plugin.AuditEntry) {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Check rotation
	if p.shouldRotate() {
		p.rotate()
	}

	// Write JSON line
	data, err := json.Marshal(entry)
	if err != nil {
		// Log to stderr but never fail
		fmt.Fprintf(os.Stderr, "audit marshal error: %v\n", err)
		return
	}

	data = append(data, '\n')
	if _, err := p.currentLog.Write(data); err != nil {
		fmt.Fprintf(os.Stderr, "audit write error: %v\n", err)
	}
}

func (p *FileAuditPlugin) shouldRotate() bool {
	info, err := p.currentLog.Stat()
	if err != nil {
		return false
	}
	return info.Size() > p.rotateSize
}

func (p *FileAuditPlugin) rotate() {
	p.currentLog.Close()
	p.openLogFile()
}

func (p *FileAuditPlugin) Query(ctx context.Context, filter *plugin.AuditFilter) ([]plugin.AuditEntry, error) {
	// Read and filter log files
	entries := make([]plugin.AuditEntry, 0)

	files, err := filepath.Glob(filepath.Join(p.logDir, "audit-*.jsonl"))
	if err != nil {
		return nil, err
	}

	for _, file := range files {
		fileEntries, err := p.readLogFile(file, filter)
		if err != nil {
			continue // Skip problematic files
		}
		entries = append(entries, fileEntries...)
	}

	return entries, nil
}

func (p *FileAuditPlugin) readLogFile(path string, filter *plugin.AuditFilter) ([]plugin.AuditEntry, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	lines := bytes.Split(data, []byte{'\n'})
	entries := make([]plugin.AuditEntry, 0)

	for _, line := range lines {
		if len(line) == 0 {
			continue
		}

		var entry plugin.AuditEntry
		if err := json.Unmarshal(line, &entry); err != nil {
			continue
		}

		if p.matchesFilter(&entry, filter) {
			entries = append(entries, entry)
		}
	}

	return entries, nil
}

func (p *FileAuditPlugin) matchesFilter(entry *plugin.AuditEntry, filter *plugin.AuditFilter) bool {
	if filter == nil {
		return true
	}

	if filter.AgentID != "" && entry.AgentID != filter.AgentID {
		return false
	}

	if filter.StartTime != nil && entry.Timestamp.Before(*filter.StartTime) {
		return false
	}

	if filter.EndTime != nil && entry.Timestamp.After(*filter.EndTime) {
		return false
	}

	return true
}

func (p *FileAuditPlugin) Shutdown(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.currentLog != nil {
		return p.currentLog.Close()
	}
	return nil
}
