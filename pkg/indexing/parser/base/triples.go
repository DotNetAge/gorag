package base

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	chat "github.com/DotNetAge/gochat/pkg/core"
	"github.com/DotNetAge/gorag/pkg/core"
)

// Triple represents a relationship between two entities.
type Triple struct {
	Subject     string `json:"subject"`
	Predicate   string `json:"predicate"`
	Object      string `json:"object"`
	SubjectType string `json:"subject_type"`
	ObjectType  string `json:"object_type"`
}

const triplesExtractionPrompt = `You are an expert in information extraction and knowledge graph construction.
Your task is to extract all significant entities and their relationships from the provided text.

[Constraints]
1. Extract only clearly stated facts.
2. Relationships (predicates) should be concise (1-3 words).
3. Identify the type for each entity (e.g., Person, Organization, Location, Concept, Technology).
4. Output MUST be a valid JSON array of objects.

[Output Format]
[
  {"subject": "Entity A", "subject_type": "Type", "predicate": "relationship", "object": "Entity B", "object_type": "Type"}
]

[Text to Process]
{{.Text}}

JSON Output:`

// TriplesExtractor uses an LLM to extract knowledge triples from text.
type TriplesExtractor struct {
	llm chat.Client
}

func NewTriplesExtractor(llm chat.Client) *TriplesExtractor {
	return &TriplesExtractor{llm: llm}
}

func (e *TriplesExtractor) Extract(ctx context.Context, text string) ([]Triple, error) {
	prompt := strings.Replace(triplesExtractionPrompt, "{{.Text}}", text, 1)
	
	messages := []chat.Message{
		chat.NewUserMessage(prompt),
	}

	resp, err := e.llm.Chat(ctx, messages)
	if err != nil {
		return nil, fmt.Errorf("LLM extraction failed: %w", err)
	}

	// Clean JSON response (sometimes LLMs wrap it in markdown blocks)
	cleanJSON := resp.Content
	if strings.HasPrefix(cleanJSON, "```json") {
		cleanJSON = strings.TrimPrefix(cleanJSON, "```json")
		cleanJSON = strings.TrimSuffix(cleanJSON, "```")
	} else if strings.HasPrefix(cleanJSON, "```") {
		cleanJSON = strings.TrimPrefix(cleanJSON, "```")
		cleanJSON = strings.TrimSuffix(cleanJSON, "```")
	}
	cleanJSON = strings.TrimSpace(cleanJSON)

	var triples []Triple
	if err := json.Unmarshal([]byte(cleanJSON), &triples); err != nil {
		return nil, fmt.Errorf("failed to parse triples JSON: %w\nContent: %s", err, cleanJSON)
	}

	return triples, nil
}
