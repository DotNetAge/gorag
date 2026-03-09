package debug

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"runtime/pprof"
	"sync"
	"time"
)

// Debugger provides debugging utilities for the RAG engine
type Debugger struct {
	enabled bool
	mu      sync.RWMutex
	logs    []DebugLog
}

// DebugLog represents a debug log entry
type DebugLog struct {
	Timestamp time.Time
	Level     string
	Message   string
	Fields    map[string]interface{}
}

// NewDebugger creates a new debugger
func NewDebugger() *Debugger {
	return &Debugger{
		enabled: false,
		logs:    make([]DebugLog, 0),
	}
}

// Enable enables debugging
func (d *Debugger) Enable() {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.enabled = true
}

// Disable disables debugging
func (d *Debugger) Disable() {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.enabled = false
}

// IsEnabled returns whether debugging is enabled
func (d *Debugger) IsEnabled() bool {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.enabled
}

// Log logs a debug message
func (d *Debugger) Log(level string, message string, fields map[string]interface{}) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if !d.enabled {
		return
	}

	log := DebugLog{
		Timestamp: time.Now(),
		Level:     level,
		Message:   message,
		Fields:    fields,
	}

	d.logs = append(d.logs, log)

	// Print to stderr
	fmt.Fprintf(os.Stderr, "[%s] %s: %s", log.Timestamp.Format(time.RFC3339), level, message)
	if len(fields) > 0 {
		fmt.Fprintf(os.Stderr, " %v", fields)
	}
	fmt.Fprintln(os.Stderr)
}

// GetLogs returns all debug logs
func (d *Debugger) GetLogs() []DebugLog {
	d.mu.RLock()
	defer d.mu.RUnlock()

	logs := make([]DebugLog, len(d.logs))
	copy(logs, d.logs)
	return logs
}

// ClearLogs clears all debug logs
func (d *Debugger) ClearLogs() {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.logs = make([]DebugLog, 0)
}

// ProfileType represents the type of profile
type ProfileType string

const (
	ProfileCPU    ProfileType = "cpu"
	ProfileMemory ProfileType = "memory"
	ProfileGoroutine ProfileType = "goroutine"
)

// StartProfile starts profiling
func (d *Debugger) StartProfile(profileType ProfileType, filename string) (func(), error) {
	switch profileType {
	case ProfileCPU:
		f, err := os.Create(filename)
		if err != nil {
			return nil, fmt.Errorf("failed to create CPU profile file: %w", err)
		}
		if err := pprof.StartCPUProfile(f); err != nil {
			f.Close()
			return nil, fmt.Errorf("failed to start CPU profile: %w", err)
		}
		return func() {
			pprof.StopCPUProfile()
			f.Close()
		}, nil

	case ProfileMemory:
		return func() {
			f, err := os.Create(filename)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Failed to create memory profile file: %v\n", err)
				return
			}
			defer f.Close()
			runtime.GC()
			if err := pprof.WriteHeapProfile(f); err != nil {
				fmt.Fprintf(os.Stderr, "Failed to write memory profile: %v\n", err)
			}
		}, nil

	case ProfileGoroutine:
		return func() {
			f, err := os.Create(filename)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Failed to create goroutine profile file: %v\n", err)
				return
			}
			defer f.Close()
			if err := pprof.Lookup("goroutine").WriteTo(f, 0); err != nil {
				fmt.Fprintf(os.Stderr, "Failed to write goroutine profile: %v\n", err)
			}
		}, nil

	default:
		return nil, fmt.Errorf("unknown profile type: %s", profileType)
	}
}

// PrintStats prints runtime statistics
func (d *Debugger) PrintStats() {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	fmt.Fprintf(os.Stderr, "\n=== Runtime Statistics ===\n")
	fmt.Fprintf(os.Stderr, "Goroutines: %d\n", runtime.NumGoroutine())
	fmt.Fprintf(os.Stderr, "Memory Alloc: %d MB\n", m.Alloc/1024/1024)
	fmt.Fprintf(os.Stderr, "Memory TotalAlloc: %d MB\n", m.TotalAlloc/1024/1024)
	fmt.Fprintf(os.Stderr, "Memory Sys: %d MB\n", m.Sys/1024/1024)
	fmt.Fprintf(os.Stderr, "GC Runs: %d\n", m.NumGC)
	fmt.Fprintf(os.Stderr, "========================\n\n")
}

// TraceFunction traces function execution time
func (d *Debugger) TraceFunction(name string) func() {
	start := time.Now()
	d.Log("DEBUG", fmt.Sprintf("Entering function: %s", name), nil)
	return func() {
		d.Log("DEBUG", fmt.Sprintf("Exiting function: %s (took %v)", name, time.Since(start)), nil)
	}
}

// Global debugger instance
var globalDebugger = NewDebugger()

// Enable enables global debugging
func Enable() {
	globalDebugger.Enable()
}

// Disable disables global debugging
func Disable() {
	globalDebugger.Disable()
}

// IsEnabled returns whether global debugging is enabled
func IsEnabled() bool {
	return globalDebugger.IsEnabled()
}

// Log logs a message to the global debugger
func Log(level string, message string, fields map[string]interface{}) {
	globalDebugger.Log(level, message, fields)
}

// Trace traces function execution time globally
func Trace(name string) func() {
	return globalDebugger.TraceFunction(name)
}

// PrintStats prints runtime statistics globally
func PrintStats() {
	globalDebugger.PrintStats()
}

// GetLogs returns all debug logs from the global debugger
func GetLogs() []DebugLog {
	return globalDebugger.GetLogs()
}

// ClearLogs clears all debug logs from the global debugger
func ClearLogs() {
	globalDebugger.ClearLogs()
}

// ContextKey is the key for storing debugger in context
type ContextKey struct{}

// WithContext returns a context with the debugger
func WithContext(ctx context.Context) context.Context {
	return context.WithValue(ctx, ContextKey{}, globalDebugger)
}

// FromContext gets the debugger from context
func FromContext(ctx context.Context) *Debugger {
	if d, ok := ctx.Value(ContextKey{}).(*Debugger); ok {
		return d
	}
	return globalDebugger
}
