package retrieval

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseAgentDecision_JSON(t *testing.T) {
	tests := []struct {
		name        string
		response    string
		wantAction  string
		wantQuery   string
		wantError   bool
	}{
		{
			name: "valid retrieve decision",
			response: `{
				"action": "retrieve",
				"query": "AI trends 2024",
				"reasoning": "Need more information about AI trends",
				"confidence": 0.9
			}`,
			wantAction: "retrieve",
			wantQuery:  "AI trends 2024",
			wantError:  false,
		},
		{
			name: "valid finish decision",
			response: `{
				"action": "finish",
				"query": "",
				"reasoning": "Have enough information",
				"confidence": 0.95
			}`,
			wantAction: "finish",
			wantQuery:  "",
			wantError:  false,
		},
		{
			name: "JSON with surrounding text",
			response: `Based on the analysis, here is my decision:
			{
				"action": "retrieve",
				"query": "machine learning applications",
				"reasoning": "Need specific examples",
				"confidence": 0.85
			}
			This should help us gather more information.`,
			wantAction: "retrieve",
			wantQuery:  "machine learning applications",
			wantError:  false,
		},
		{
			name: "invalid action",
			response: `{
				"action": "invalid_action",
				"query": "test",
				"reasoning": "test",
				"confidence": 0.8
			}`,
			wantError: true,
		},
		{
			name: "missing query for retrieve",
			response: `{
				"action": "retrieve",
				"query": "",
				"reasoning": "test",
				"confidence": 0.8
			}`,
			wantError: true,
		},
		{
			name: "invalid confidence",
			response: `{
				"action": "finish",
				"query": "",
				"reasoning": "test",
				"confidence": 1.5
			}`,
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			decision, err := ParseAgentDecision(tt.response)

			if tt.wantError {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.wantAction, decision.Action)
			assert.Equal(t, tt.wantQuery, decision.Query)
			assert.NotEmpty(t, decision.Reasoning)
			assert.GreaterOrEqual(t, decision.Confidence, 0.0)
			assert.LessOrEqual(t, decision.Confidence, 1.0)
		})
	}
}

func TestParseAgentDecision_Text(t *testing.T) {
	tests := []struct {
		name        string
		response    string
		wantAction  string
		wantQuery   string
		wantError   bool
	}{
		{
			name: "text format retrieve",
			response: `action: retrieve
query: deep learning frameworks
reasoning: Need to find information about popular frameworks
confidence: 0.8`,
			wantAction: "retrieve",
			wantQuery:  "deep learning frameworks",
			wantError:  false,
		},
		{
			name: "text format finish",
			response: `action: finish
reasoning: We have sufficient information to answer the question
confidence: 0.9`,
			wantAction: "finish",
			wantError:  false,
		},
		{
			name: "case insensitive",
			response: `Action: RETRIEVE
Query: test query
Reasoning: test reasoning`,
			wantAction: "retrieve",
			wantQuery:  "test query",
			wantError:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			decision, err := ParseAgentDecision(tt.response)

			if tt.wantError {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.wantAction, decision.Action)
			if tt.wantQuery != "" {
				assert.Equal(t, tt.wantQuery, decision.Query)
			}
		})
	}
}

func TestAgentDecision_Validate(t *testing.T) {
	tests := []struct {
		name      string
		decision  AgentDecision
		wantError bool
	}{
		{
			name: "valid retrieve",
			decision: AgentDecision{
				Action:     "retrieve",
				Query:      "test query",
				Reasoning:  "test",
				Confidence: 0.8,
			},
			wantError: false,
		},
		{
			name: "valid finish",
			decision: AgentDecision{
				Action:     "finish",
				Reasoning:  "test",
				Confidence: 0.9,
			},
			wantError: false,
		},
		{
			name: "invalid action",
			decision: AgentDecision{
				Action:     "invalid",
				Confidence: 0.8,
			},
			wantError: true,
		},
		{
			name: "retrieve without query",
			decision: AgentDecision{
				Action:     "retrieve",
				Query:      "",
				Confidence: 0.8,
			},
			wantError: true,
		},
		{
			name: "confidence too low",
			decision: AgentDecision{
				Action:     "finish",
				Confidence: -0.1,
			},
			wantError: true,
		},
		{
			name: "confidence too high",
			decision: AgentDecision{
				Action:     "finish",
				Confidence: 1.1,
			},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.decision.Validate()
			if tt.wantError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestAgentReflection_Validate(t *testing.T) {
	tests := []struct {
		name       string
		reflection AgentReflection
		wantError  bool
	}{
		{
			name: "valid with enough info",
			reflection: AgentReflection{
				HasEnoughInfo: true,
				Confidence:    0.9,
			},
			wantError: false,
		},
		{
			name: "valid without enough info",
			reflection: AgentReflection{
				HasEnoughInfo:    false,
				MissingInfo:      []string{"info1", "info2"},
				RecommendedQuery: "test query",
				Confidence:       0.8,
			},
			wantError: false,
		},
		{
			name: "missing recommended query",
			reflection: AgentReflection{
				HasEnoughInfo: false,
				MissingInfo:   []string{"info1"},
				Confidence:    0.8,
			},
			wantError: true,
		},
		{
			name: "invalid confidence",
			reflection: AgentReflection{
				HasEnoughInfo: true,
				Confidence:    1.5,
			},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.reflection.Validate()
			if tt.wantError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
