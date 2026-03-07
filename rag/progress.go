package rag

import (
	"context"
	"sync"
	"time"
)

// ProgressStatus represents the status of an operation
type ProgressStatus string

const (
	// ProgressStatusPending indicates the operation is pending
	ProgressStatusPending ProgressStatus = "pending"
	// ProgressStatusRunning indicates the operation is running
	ProgressStatusRunning ProgressStatus = "running"
	// ProgressStatusCompleted indicates the operation completed successfully
	ProgressStatusCompleted ProgressStatus = "completed"
	// ProgressStatusFailed indicates the operation failed
	ProgressStatusFailed ProgressStatus = "failed"
	// ProgressStatusCancelled indicates the operation was cancelled
	ProgressStatusCancelled ProgressStatus = "cancelled"
)

// Progress represents the progress of an indexing operation
type Progress struct {
	ID             string         `json:"id"`
	Status         ProgressStatus `json:"status"`
	TotalFiles     int            `json:"total_files"`
	ProcessedFiles int            `json:"processed_files"`
	FailedFiles    []string       `json:"failed_files"`
	CurrentFile    string         `json:"current_file"`
	StartTime      time.Time      `json:"start_time"`
	EndTime        *time.Time     `json:"end_time,omitempty"`
	Error          string         `json:"error,omitempty"`

	mu sync.RWMutex
}

// NewProgress creates a new progress tracker
func NewProgress(id string, totalFiles int) *Progress {
	return &Progress{
		ID:          id,
		Status:      ProgressStatusPending,
		TotalFiles:  totalFiles,
		FailedFiles: []string{},
		StartTime:   time.Now(),
	}
}

// Start marks the progress as running
func (p *Progress) Start() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.Status = ProgressStatusRunning
}

// IncrementProcessed increments the processed files count
func (p *Progress) IncrementProcessed() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.ProcessedFiles++
}

// SetCurrentFile sets the current file being processed
func (p *Progress) SetCurrentFile(file string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.CurrentFile = file
}

// AddFailedFile adds a failed file to the list
func (p *Progress) AddFailedFile(file string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.FailedFiles = append(p.FailedFiles, file)
}

// Complete marks the progress as completed
func (p *Progress) Complete() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.Status = ProgressStatusCompleted
	now := time.Now()
	p.EndTime = &now
	p.CurrentFile = ""
}

// Fail marks the progress as failed
func (p *Progress) Fail(err error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.Status = ProgressStatusFailed
	now := time.Now()
	p.EndTime = &now
	if err != nil {
		p.Error = err.Error()
	}
}

// Cancel marks the progress as cancelled
func (p *Progress) Cancel() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.Status = ProgressStatusCancelled
	now := time.Now()
	p.EndTime = &now
}

// Percentage returns the completion percentage
func (p *Progress) Percentage() float64 {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if p.TotalFiles == 0 {
		return 0
	}
	return float64(p.ProcessedFiles) / float64(p.TotalFiles) * 100
}

// Duration returns the elapsed time
func (p *Progress) Duration() time.Duration {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if p.EndTime != nil {
		return p.EndTime.Sub(p.StartTime)
	}
	return time.Since(p.StartTime)
}

// IsComplete returns true if the operation is complete
func (p *Progress) IsComplete() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.Status == ProgressStatusCompleted ||
	       p.Status == ProgressStatusFailed ||
	       p.Status == ProgressStatusCancelled
}

// ProgressSnapshot is a read-only copy of Progress state (safe to copy by value)
type ProgressSnapshot struct {
	ID             string         `json:"id"`
	Status         ProgressStatus `json:"status"`
	TotalFiles     int            `json:"total_files"`
	ProcessedFiles int            `json:"processed_files"`
	FailedFiles    []string       `json:"failed_files"`
	CurrentFile    string         `json:"current_file"`
	StartTime      time.Time      `json:"start_time"`
	EndTime        *time.Time     `json:"end_time,omitempty"`
	Error          string         `json:"error,omitempty"`
}

// Snapshot returns a copy of the current progress state
func (p *Progress) Snapshot() ProgressSnapshot {
	p.mu.RLock()
	defer p.mu.RUnlock()

	snapshot := ProgressSnapshot{
		ID:             p.ID,
		Status:         p.Status,
		TotalFiles:     p.TotalFiles,
		ProcessedFiles: p.ProcessedFiles,
		FailedFiles:    make([]string, len(p.FailedFiles)),
		CurrentFile:    p.CurrentFile,
		StartTime:      p.StartTime,
		Error:          p.Error,
	}

	copy(snapshot.FailedFiles, p.FailedFiles)

	if p.EndTime != nil {
		endTime := *p.EndTime
		snapshot.EndTime = &endTime
	}

	return snapshot
}

// ProgressTracker manages multiple progress instances
type ProgressTracker struct {
	progresses map[string]*Progress
	mu         sync.RWMutex
}

// NewProgressTracker creates a new progress tracker
func NewProgressTracker() *ProgressTracker {
	return &ProgressTracker{
		progresses: make(map[string]*Progress),
	}
}

// Create creates a new progress instance
func (t *ProgressTracker) Create(id string, totalFiles int) *Progress {
	t.mu.Lock()
	defer t.mu.Unlock()

	progress := NewProgress(id, totalFiles)
	t.progresses[id] = progress
	return progress
}

// Get retrieves a progress instance by ID
func (t *ProgressTracker) Get(id string) (*Progress, bool) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	progress, exists := t.progresses[id]
	return progress, exists
}

// Delete removes a progress instance
func (t *ProgressTracker) Delete(id string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	delete(t.progresses, id)
}

// List returns all progress instances
func (t *ProgressTracker) List() []ProgressSnapshot {
	t.mu.RLock()
	defer t.mu.RUnlock()

	snapshots := make([]ProgressSnapshot, 0, len(t.progresses))
	for _, p := range t.progresses {
		snapshots = append(snapshots, p.Snapshot())
	}
	return snapshots
}

// Cleanup removes completed progress instances older than the specified duration
func (t *ProgressTracker) Cleanup(maxAge time.Duration) int {
	t.mu.Lock()
	defer t.mu.Unlock()

	now := time.Now()
	removed := 0

	for id, p := range t.progresses {
		if p.IsComplete() && p.EndTime != nil && now.Sub(*p.EndTime) > maxAge {
			delete(t.progresses, id)
			removed++
		}
	}

	return removed
}

// ProgressChannel wraps a progress instance with a channel for updates
type ProgressChannel struct {
	Progress *Progress
	Updates  chan ProgressSnapshot
	ctx      context.Context
	cancel   context.CancelFunc
}

// NewProgressChannel creates a new progress channel
func NewProgressChannel(ctx context.Context, progress *Progress) *ProgressChannel {
	ctx, cancel := context.WithCancel(ctx)
	return &ProgressChannel{
		Progress: progress,
		Updates:  make(chan ProgressSnapshot, 10),
		ctx:      ctx,
		cancel:   cancel,
	}
}

// Send sends a progress update
func (pc *ProgressChannel) Send() {
	select {
	case pc.Updates <- pc.Progress.Snapshot():
	case <-pc.ctx.Done():
	default:
		// Channel full, skip update
	}
}

// Close closes the progress channel
func (pc *ProgressChannel) Close() {
	pc.cancel()
	close(pc.Updates)
}

// Watch returns a channel that receives progress updates
func (pc *ProgressChannel) Watch() <-chan ProgressSnapshot {
	return pc.Updates
}
