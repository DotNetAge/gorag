package rag

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNewProgress(t *testing.T) {
	p := NewProgress("test-id", 10)

	assert.Equal(t, "test-id", p.ID)
	assert.Equal(t, ProgressStatusPending, p.Status)
	assert.Equal(t, 10, p.TotalFiles)
	assert.Equal(t, 0, p.ProcessedFiles)
	assert.Empty(t, p.FailedFiles)
	assert.Empty(t, p.CurrentFile)
	assert.Nil(t, p.EndTime)
}

func TestProgress_Start(t *testing.T) {
	p := NewProgress("test-id", 10)
	p.Start()

	assert.Equal(t, ProgressStatusRunning, p.Status)
}

func TestProgress_IncrementProcessed(t *testing.T) {
	p := NewProgress("test-id", 10)

	assert.Equal(t, 0, p.ProcessedFiles)

	p.IncrementProcessed()
	assert.Equal(t, 1, p.ProcessedFiles)

	p.IncrementProcessed()
	assert.Equal(t, 2, p.ProcessedFiles)
}

func TestProgress_SetCurrentFile(t *testing.T) {
	p := NewProgress("test-id", 10)

	p.SetCurrentFile("file1.txt")
	assert.Equal(t, "file1.txt", p.CurrentFile)

	p.SetCurrentFile("file2.txt")
	assert.Equal(t, "file2.txt", p.CurrentFile)
}

func TestProgress_AddFailedFile(t *testing.T) {
	p := NewProgress("test-id", 10)

	assert.Empty(t, p.FailedFiles)

	p.AddFailedFile("failed1.txt")
	assert.Len(t, p.FailedFiles, 1)
	assert.Contains(t, p.FailedFiles, "failed1.txt")

	p.AddFailedFile("failed2.txt")
	assert.Len(t, p.FailedFiles, 2)
	assert.Contains(t, p.FailedFiles, "failed2.txt")
}

func TestProgress_Complete(t *testing.T) {
	p := NewProgress("test-id", 10)
	p.Start()
	p.SetCurrentFile("file.txt")

	p.Complete()

	assert.Equal(t, ProgressStatusCompleted, p.Status)
	assert.NotNil(t, p.EndTime)
	assert.Empty(t, p.CurrentFile)
}

func TestProgress_Fail(t *testing.T) {
	p := NewProgress("test-id", 10)
	p.Start()

	err := assert.AnError
	p.Fail(err)

	assert.Equal(t, ProgressStatusFailed, p.Status)
	assert.NotNil(t, p.EndTime)
	assert.Equal(t, err.Error(), p.Error)
}

func TestProgress_Cancel(t *testing.T) {
	p := NewProgress("test-id", 10)
	p.Start()

	p.Cancel()

	assert.Equal(t, ProgressStatusCancelled, p.Status)
	assert.NotNil(t, p.EndTime)
}

func TestProgress_Percentage(t *testing.T) {
	tests := []struct {
		name       string
		total      int
		processed  int
		expected   float64
	}{
		{
			name:      "0%",
			total:     10,
			processed: 0,
			expected:  0,
		},
		{
			name:      "50%",
			total:     10,
			processed: 5,
			expected:  50,
		},
		{
			name:      "100%",
			total:     10,
			processed: 10,
			expected:  100,
		},
		{
			name:      "zero total",
			total:     0,
			processed: 0,
			expected:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := NewProgress("test-id", tt.total)
			for i := 0; i < tt.processed; i++ {
				p.IncrementProcessed()
			}

			assert.InDelta(t, tt.expected, p.Percentage(), 0.01)
		})
	}
}

func TestProgress_Duration(t *testing.T) {
	p := NewProgress("test-id", 10)
	p.Start()

	time.Sleep(100 * time.Millisecond)

	duration := p.Duration()
	assert.Greater(t, duration, 100*time.Millisecond)

	// Complete and check duration is fixed
	p.Complete()
	duration1 := p.Duration()
	time.Sleep(50 * time.Millisecond)
	duration2 := p.Duration()

	assert.Equal(t, duration1, duration2)
}

func TestProgress_IsComplete(t *testing.T) {
	tests := []struct {
		name     string
		status   ProgressStatus
		expected bool
	}{
		{
			name:     "pending",
			status:   ProgressStatusPending,
			expected: false,
		},
		{
			name:     "running",
			status:   ProgressStatusRunning,
			expected: false,
		},
		{
			name:     "completed",
			status:   ProgressStatusCompleted,
			expected: true,
		},
		{
			name:     "failed",
			status:   ProgressStatusFailed,
			expected: true,
		},
		{
			name:     "cancelled",
			status:   ProgressStatusCancelled,
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := NewProgress("test-id", 10)
			p.Status = tt.status

			assert.Equal(t, tt.expected, p.IsComplete())
		})
	}
}

func TestProgress_Snapshot(t *testing.T) {
	p := NewProgress("test-id", 10)
	p.Start()
	p.SetCurrentFile("file.txt")
	p.IncrementProcessed()
	p.AddFailedFile("failed.txt")

	snapshot := p.Snapshot()

	assert.Equal(t, p.ID, snapshot.ID)
	assert.Equal(t, p.Status, snapshot.Status)
	assert.Equal(t, p.TotalFiles, snapshot.TotalFiles)
	assert.Equal(t, p.ProcessedFiles, snapshot.ProcessedFiles)
	assert.Equal(t, p.CurrentFile, snapshot.CurrentFile)
	assert.Equal(t, p.FailedFiles, snapshot.FailedFiles)

	// Modify original should not affect snapshot
	p.IncrementProcessed()
	assert.NotEqual(t, p.ProcessedFiles, snapshot.ProcessedFiles)
}

func TestProgressTracker_Create(t *testing.T) {
	tracker := NewProgressTracker()

	p := tracker.Create("test-id", 10)

	assert.NotNil(t, p)
	assert.Equal(t, "test-id", p.ID)
	assert.Equal(t, 10, p.TotalFiles)
}

func TestProgressTracker_Get(t *testing.T) {
	tracker := NewProgressTracker()

	// Get non-existent
	_, exists := tracker.Get("non-existent")
	assert.False(t, exists)

	// Create and get
	tracker.Create("test-id", 10)
	p, exists := tracker.Get("test-id")
	assert.True(t, exists)
	assert.NotNil(t, p)
	assert.Equal(t, "test-id", p.ID)
}

func TestProgressTracker_Delete(t *testing.T) {
	tracker := NewProgressTracker()

	tracker.Create("test-id", 10)
	_, exists := tracker.Get("test-id")
	assert.True(t, exists)

	tracker.Delete("test-id")
	_, exists = tracker.Get("test-id")
	assert.False(t, exists)
}

func TestProgressTracker_List(t *testing.T) {
	tracker := NewProgressTracker()

	// Empty list
	list := tracker.List()
	assert.Empty(t, list)

	// Create multiple
	tracker.Create("id1", 10)
	tracker.Create("id2", 20)
	tracker.Create("id3", 30)

	list = tracker.List()
	assert.Len(t, list, 3)
}

func TestProgressTracker_Cleanup(t *testing.T) {
	tracker := NewProgressTracker()

	// Create completed progress (old)
	p1 := tracker.Create("old-completed", 10)
	p1.Complete()
	oldTime := time.Now().Add(-2 * time.Hour)
	p1.EndTime = &oldTime

	// Create completed progress (recent)
	p2 := tracker.Create("recent-completed", 10)
	p2.Complete()

	// Create running progress
	p3 := tracker.Create("running", 10)
	p3.Start()

	// Cleanup old completed (older than 1 hour)
	removed := tracker.Cleanup(1 * time.Hour)

	assert.Equal(t, 1, removed)

	// Verify old completed is removed
	_, exists := tracker.Get("old-completed")
	assert.False(t, exists)

	// Verify recent completed still exists
	_, exists = tracker.Get("recent-completed")
	assert.True(t, exists)

	// Verify running still exists
	_, exists = tracker.Get("running")
	assert.True(t, exists)
}

func TestProgressChannel_SendAndWatch(t *testing.T) {
	ctx := context.Background()
	p := NewProgress("test-id", 10)
	pc := NewProgressChannel(ctx, p)
	defer pc.Close()

	// Start watching
	updates := pc.Watch()

	// Send update
	p.Start()
	pc.Send()

	// Receive update
	select {
	case update := <-updates:
		assert.Equal(t, ProgressStatusRunning, update.Status)
	case <-time.After(1 * time.Second):
		t.Fatal("timeout waiting for update")
	}
}

func TestProgressChannel_Close(t *testing.T) {
	ctx := context.Background()
	p := NewProgress("test-id", 10)
	pc := NewProgressChannel(ctx, p)

	updates := pc.Watch()

	pc.Close()

	// Channel should be closed
	_, ok := <-updates
	assert.False(t, ok)
}

func TestProgress_ConcurrentAccess(t *testing.T) {
	p := NewProgress("test-id", 1000)
	p.Start()

	// Concurrent increments
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 10; j++ {
				p.IncrementProcessed()
				p.SetCurrentFile("file.txt")
				_ = p.Percentage()
				_ = p.Snapshot()
			}
		}()
	}

	wg.Wait()

	assert.Equal(t, 1000, p.ProcessedFiles)
}

func TestProgressTracker_ConcurrentAccess(t *testing.T) {
	tracker := NewProgressTracker()

	var wg sync.WaitGroup

	// Concurrent creates
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			tracker.Create(string(rune(id)), 10)
		}(i)
	}

	wg.Wait()

	list := tracker.List()
	assert.Len(t, list, 100)
}

func TestProgress_FullWorkflow(t *testing.T) {
	tracker := NewProgressTracker()

	// Create progress
	p := tracker.Create("workflow-test", 5)
	assert.Equal(t, ProgressStatusPending, p.Status)
	assert.Equal(t, 0.0, p.Percentage())

	// Start
	p.Start()
	assert.Equal(t, ProgressStatusRunning, p.Status)

	// Process files
	files := []string{"file1.txt", "file2.txt", "file3.txt", "file4.txt", "file5.txt"}
	for i, file := range files {
		p.SetCurrentFile(file)
		assert.Equal(t, file, p.CurrentFile)

		// Simulate processing
		time.Sleep(10 * time.Millisecond)

		if i == 2 {
			// Simulate failure
			p.AddFailedFile(file)
		}

		p.IncrementProcessed()

		expectedPercentage := float64(i+1) / float64(len(files)) * 100
		assert.InDelta(t, expectedPercentage, p.Percentage(), 0.01)
	}

	// Complete
	p.Complete()
	assert.Equal(t, ProgressStatusCompleted, p.Status)
	assert.Equal(t, 100.0, p.Percentage())
	assert.Len(t, p.FailedFiles, 1)
	assert.Contains(t, p.FailedFiles, "file3.txt")
	assert.NotNil(t, p.EndTime)
	assert.Greater(t, p.Duration(), time.Duration(0))

	// Verify snapshot
	snapshot := p.Snapshot()
	assert.Equal(t, p.ID, snapshot.ID)
	assert.Equal(t, p.Status, snapshot.Status)
	assert.Equal(t, p.ProcessedFiles, snapshot.ProcessedFiles)
}
