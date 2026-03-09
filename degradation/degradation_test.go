package degradation

import (
	"errors"
	"testing"
	"time"
)

func TestDegradationManager(t *testing.T) {
	// Create degradation manager
	manager := New(Config{
		EnableReducedFeatures: true,
		EnableMinimal:         true,
		EnableFallback:        true,
		ErrorThreshold:        1,
		LatencyThreshold:      10 * time.Millisecond,
	})

	// Test 1: Initial level
	level := manager.GetLevel()
	if level != LevelNormal {
		t.Errorf("Expected initial level to be LevelNormal, got %v", level)
	}

	// Test 2: Record error and check degradation
	manager.RecordError(errors.New("test error"))
	level = manager.GetLevel()
	if level != LevelReducedFeatures {
		t.Errorf("Expected level to be LevelReducedFeatures after 1 error, got %v", level)
	}

	// Test 3: Record another error and check further degradation
	manager.RecordError(errors.New("test error"))
	level = manager.GetLevel()
	if level != LevelMinimal {
		t.Errorf("Expected level to be LevelMinimal after 2 errors, got %v", level)
	}

	// Test 4: Record third error and check fallback level
	manager.RecordError(errors.New("test error"))
	level = manager.GetLevel()
	if level != LevelFallback {
		t.Errorf("Expected level to be LevelFallback after 3 errors, got %v", level)
	}

	// Test 5: Record latency
	manager.Reset()
	manager.RecordLatency(20 * time.Millisecond) // Above threshold
	level = manager.GetLevel()
	if level != LevelReducedFeatures {
		t.Errorf("Expected level to be LevelReducedFeatures after high latency, got %v", level)
	}

	// Test 6: Reset
	manager.Reset()
	level = manager.GetLevel()
	if level != LevelNormal {
		t.Errorf("Expected level to be LevelNormal after reset, got %v", level)
	}

	// Test 7: ShouldDegrade
	manager.RecordError(errors.New("test error"))
	shouldDegrade := manager.ShouldDegrade(LevelReducedFeatures)
	if !shouldDegrade {
		t.Error("Expected ShouldDegrade to return true for LevelReducedFeatures")
	}
}

func TestDegradationManagerWithSuccesses(t *testing.T) {
	// Create degradation manager
	manager := New(Config{
		EnableReducedFeatures: true,
		ErrorThreshold:        1,
	})

	// Level should still be Normal
	level := manager.GetLevel()
	if level != LevelNormal {
		t.Errorf("Expected level to be LevelNormal, got %v", level)
	}
}
