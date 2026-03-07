package retrieval

import (
	"encoding/json"
	"fmt"
	"strings"
)

// AgentDecision represents a structured decision from the agent
type AgentDecision struct {
	Action     string  `json:"action"`     // "retrieve" or "finish"
	Query      string  `json:"query"`      // Search query if action is "retrieve"
	Reasoning  string  `json:"reasoning"`  // Agent's reasoning for this decision
	Confidence float64 `json:"confidence"` // Confidence score (0.0-1.0)
}

// Validate validates the agent decision
func (d *AgentDecision) Validate() error {
	// Normalize action
	d.Action = strings.ToLower(strings.TrimSpace(d.Action))

	// Validate action
	if d.Action != "retrieve" && d.Action != "finish" {
		return fmt.Errorf("invalid action: %s (must be 'retrieve' or 'finish')", d.Action)
	}

	// Validate query for retrieve action
	if d.Action == "retrieve" && strings.TrimSpace(d.Query) == "" {
		return fmt.Errorf("query is required when action is 'retrieve'")
	}

	// Validate confidence
	if d.Confidence < 0.0 || d.Confidence > 1.0 {
		return fmt.Errorf("confidence must be between 0.0 and 1.0, got: %f", d.Confidence)
	}

	return nil
}

// ParseAgentDecision parses agent decision from LLM response
// It supports both JSON format and fallback text parsing
func ParseAgentDecision(response string) (*AgentDecision, error) {
	// Try JSON parsing first
	decision, err := parseJSON(response)
	if err == nil {
		if err := decision.Validate(); err != nil {
			return nil, fmt.Errorf("invalid decision: %w", err)
		}
		return decision, nil
	}

	// Fallback to text parsing
	decision, err = parseText(response)
	if err != nil {
		return nil, fmt.Errorf("failed to parse agent decision: %w", err)
	}

	if err := decision.Validate(); err != nil {
		return nil, fmt.Errorf("invalid decision: %w", err)
	}

	return decision, nil
}

// parseJSON attempts to parse JSON format response
func parseJSON(response string) (*AgentDecision, error) {
	// Try to find JSON object in response
	start := strings.Index(response, "{")
	end := strings.LastIndex(response, "}")

	if start == -1 || end == -1 || start >= end {
		return nil, fmt.Errorf("no JSON object found in response")
	}

	jsonStr := response[start : end+1]

	var decision AgentDecision
	if err := json.Unmarshal([]byte(jsonStr), &decision); err != nil {
		return nil, fmt.Errorf("failed to unmarshal JSON: %w", err)
	}

	return &decision, nil
}

// parseText attempts to parse text format response (fallback)
func parseText(response string) (*AgentDecision, error) {
	decision := &AgentDecision{
		Action:     "finish",
		Confidence: 0.5, // Default confidence for text parsing
	}

	lines := strings.Split(response, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)

		// Parse action
		if strings.HasPrefix(strings.ToLower(line), "action:") {
			decision.Action = strings.TrimSpace(strings.TrimPrefix(line, "action:"))
			decision.Action = strings.TrimSpace(strings.TrimPrefix(decision.Action, "Action:"))
		}

		// Parse query
		if strings.HasPrefix(strings.ToLower(line), "query:") {
			decision.Query = strings.TrimSpace(strings.TrimPrefix(line, "query:"))
			decision.Query = strings.TrimSpace(strings.TrimPrefix(decision.Query, "Query:"))
		}

		// Parse reasoning
		if strings.HasPrefix(strings.ToLower(line), "reasoning:") {
			decision.Reasoning = strings.TrimSpace(strings.TrimPrefix(line, "reasoning:"))
			decision.Reasoning = strings.TrimSpace(strings.TrimPrefix(decision.Reasoning, "Reasoning:"))
		}

		// Parse confidence
		if strings.HasPrefix(strings.ToLower(line), "confidence:") {
			confStr := strings.TrimSpace(strings.TrimPrefix(line, "confidence:"))
			confStr = strings.TrimSpace(strings.TrimPrefix(confStr, "Confidence:"))
			var conf float64
			if _, err := fmt.Sscanf(confStr, "%f", &conf); err == nil {
				decision.Confidence = conf
			}
		}
	}

	// If no reasoning found, use the entire response
	if decision.Reasoning == "" {
		decision.Reasoning = response
	}

	return decision, nil
}

// AgentReflection represents agent's reflection on the retrieval process
type AgentReflection struct {
	HasEnoughInfo     bool     `json:"has_enough_info"`      // Whether agent has enough information
	MissingInfo       []string `json:"missing_info"`         // List of missing information
	RecommendedQuery  string   `json:"recommended_query"`    // Recommended next query
	RecommendedAction string   `json:"recommended_action"`   // Recommended action
	Confidence        float64  `json:"confidence"`           // Confidence in the reflection
}

// Validate validates the agent reflection
func (r *AgentReflection) Validate() error {
	if r.Confidence < 0.0 || r.Confidence > 1.0 {
		return fmt.Errorf("confidence must be between 0.0 and 1.0, got: %f", r.Confidence)
	}

	if !r.HasEnoughInfo && r.RecommendedQuery == "" {
		return fmt.Errorf("recommended_query is required when has_enough_info is false")
	}

	return nil
}
