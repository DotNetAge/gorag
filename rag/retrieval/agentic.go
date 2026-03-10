package retrieval

import (
	"context"
	"github.com/DotNetAge/gorag/utils/llmutil"
	"fmt"
	"sort"
	"strings"

	"github.com/DotNetAge/gorag/core"
	"github.com/DotNetAge/gorag/embedding"
	gochatcore "github.com/DotNetAge/gochat/pkg/core"
	"github.com/DotNetAge/gorag/vectorstore"
)

// AgenticRAG implements agentic retrieval with autonomous decision-making
//
// Agentic RAG is the most advanced form of RAG where the retrieval process
// is embedded in an autonomous agent's reasoning loop. The agent can:
// 1. Decide if retrieval is needed based on the task
// 2. Generate appropriate search queries
// 3. Evaluate the quality of retrieval results
// 4. Decide if additional retrieval is needed
// 5. Iteratively refine the retrieval process
//
// Example use case: "Write a report comparing the latest AI trends from 2024"
// The agent would:
// 1. Analyze the task and determine it needs current information
// 2. Generate a search query for "2024 AI trends"
// 3. Evaluate the results and identify gaps
// 4. Generate more specific queries for different AI domains
// 5. Continue until it has comprehensive information
// 6. Synthesize the information into a report
type AgenticRAG struct {
	llm           gochatcore.Client
	embedder      embedding.Provider
	vectorStore   vectorstore.Store
	maxIterations int
	taskPrompt    string
	reflectPrompt string
}

// Response represents a RAG query response
type Response struct {
	Answer  string
	Sources []core.Result
}

// AgentState represents the current state of the agent
type AgentState struct {
	Task           string
	RetrievedInfo  []core.Result
	Reasoning      []string
	IterationCount int
	Completed      bool
	FinalAnswer    string
}

// NewAgenticRAG creates a new agentic RAG instance
func NewAgenticRAG(
	llm gochatcore.Client,
	embedder embedding.Provider,
	vectorStore vectorstore.Store,
) *AgenticRAG {
	return &AgenticRAG{
		llm:           llm,
		embedder:      embedder,
		vectorStore:   vectorStore,
		maxIterations: 5, // Default: maximum 5 iterations
		taskPrompt:    defaultTaskPrompt,
		reflectPrompt: defaultReflectPrompt,
	}
}

// WithMaxIterations sets the maximum number of iterations
func (a *AgenticRAG) WithMaxIterations(iterations int) *AgenticRAG {
	if iterations > 0 {
		a.maxIterations = iterations
	}
	return a
}

// WithTaskPrompt sets a custom task prompt
func (a *AgenticRAG) WithTaskPrompt(prompt string) *AgenticRAG {
	a.taskPrompt = prompt
	return a
}

// WithReflectPrompt sets a custom reflection prompt
func (a *AgenticRAG) WithReflectPrompt(prompt string) *AgenticRAG {
	a.reflectPrompt = prompt
	return a
}

// Search performs agentic retrieval
func (a *AgenticRAG) Search(ctx context.Context, task string, topK int) ([]core.Result, error) {
	// Initialize agent state
	state := &AgentState{
		Task:           task,
		RetrievedInfo:  []core.Result{},
		Reasoning:      []string{},
		IterationCount: 0,
		Completed:      false,
	}

	// Agent loop
	for state.IterationCount < a.maxIterations && !state.Completed {
		// Step 1: Analyze task and decide on next action
		action, query, reasoning := a.analyzeTask(ctx, state)
		state.Reasoning = append(state.Reasoning, reasoning)

		if action == "retrieve" && query != "" {
			// Step 2: Perform retrieval
			results, err := a.performRetrieval(ctx, query, topK)
			if err == nil {
				state.RetrievedInfo = append(state.RetrievedInfo, results...)
			}
		} else if action == "finish" {
			// Step 3: Task completed
			state.Completed = true
			break
		}

		state.IterationCount++
	}

	// Return the collected information
	return a.deduplicateAndSort(state.RetrievedInfo, topK), nil
}

// Query performs an agentic RAG query
func (a *AgenticRAG) Query(ctx context.Context, task string, instructions string, promptTemplate string) (*Response, error) {
	// Perform agentic search
	results, err := a.Search(ctx, task, 5) // Default topK: 5
	if err != nil {
		return nil, err
	}

	// Build context from results
	contexts := make([]string, len(results))
	for i, result := range results {
		contexts[i] = result.Content
	}

	// Build prompt
	prompt := buildAgenticPrompt(task, contexts, instructions, promptTemplate)

	// Generate answer
	answer, err := llmutil.Complete(ctx, a.llm, prompt)
	if err != nil {
		return nil, err
	}

	return &Response{
		Answer:  answer,
		Sources: results,
	}, nil
}

// buildAgenticPrompt builds the prompt for agentic RAG
func buildAgenticPrompt(task string, contexts []string, instructions string, template string) string {
	if template != "" {
		// Build context string
		var contextStr strings.Builder
		for i, ctx := range contexts {
			contextStr.WriteString(fmt.Sprintf("%d. %s\n", i+1, ctx))
		}

		// Replace placeholders
		result := strings.ReplaceAll(template, "{task}", task)
		result = strings.ReplaceAll(result, "{context}", contextStr.String())
		if instructions != "" {
			result = strings.ReplaceAll(result, "{instructions}", instructions)
		}
		return result
	}

	var buf strings.Builder
	buf.WriteString("基于以下上下文信息完成任务。如果上下文中没有相关信息，请说明无法完成。\n\n")
	if instructions != "" {
		buf.WriteString("指令：\n")
		buf.WriteString(instructions)
		buf.WriteString("\n\n")
	}
	buf.WriteString("上下文：\n")

	for i, ctx := range contexts {
		buf.WriteString(fmt.Sprintf("%d. %s\n", i+1, ctx))
	}

	buf.WriteString("\n任务：\n")
	buf.WriteString(task)
	buf.WriteString("\n\n结果：\n")

	return buf.String()
}

// analyzeTask analyzes the current task and state to determine next action
func (a *AgenticRAG) analyzeTask(ctx context.Context, state *AgentState) (action string, query string, reasoning string) {
	// Build task analysis prompt
	prompt := a.taskPrompt
	prompt = strings.Replace(prompt, "{task}", state.Task, 1)

	// Add retrieved information
	retrievedInfo := ""
	if len(state.RetrievedInfo) > 0 {
		retrievedInfo = "Retrieved information so far:\n"
		for i, result := range state.RetrievedInfo[:min(5, len(state.RetrievedInfo))] {
			retrievedInfo += fmt.Sprintf("%d. %s\n", i+1, result.Content)
		}
	}
	prompt = strings.Replace(prompt, "{retrieved_info}", retrievedInfo, 1)

	// Add previous reasoning
	reasoningHistory := ""
	if len(state.Reasoning) > 0 {
		reasoningHistory = "Previous reasoning:\n"
		for i, r := range state.Reasoning {
			reasoningHistory += fmt.Sprintf("%d. %s\n", i+1, r)
		}
	}
	prompt = strings.Replace(prompt, "{reasoning_history}", reasoningHistory, 1)

	// Get analysis from LLM
	response, err := llmutil.Complete(ctx, a.llm, prompt)
	if err != nil {
		return "finish", "", "Error analyzing task"
	}

	// Parse response using structured parser
	decision, err := ParseAgentDecision(response)
	if err != nil {
		// Fallback to finish if parsing fails
		return "finish", "", fmt.Sprintf("Failed to parse decision: %v", err)
	}

	return decision.Action, decision.Query, decision.Reasoning
}

// performRetrieval performs the actual retrieval based on the generated query
func (a *AgenticRAG) performRetrieval(ctx context.Context, query string, topK int) ([]core.Result, error) {
	// Get embedding for query
	embeddings, err := a.embedder.Embed(ctx, []string{query})
	if err != nil {
		return nil, err
	}

	// Search vector store
	searchOpts := vectorstore.SearchOptions{
		TopK: topK * 2, // Get more results for evaluation
	}
	return a.vectorStore.Search(ctx, embeddings[0], searchOpts)
}


// deduplicateAndSort deduplicates and sorts results by score
func (a *AgenticRAG) deduplicateAndSort(results []core.Result, topK int) []core.Result {
	// Deduplicate results
	seen := make(map[string]bool)
	var uniqueResults []core.Result

	for _, result := range results {
		if !seen[result.ID] {
			seen[result.ID] = true
			uniqueResults = append(uniqueResults, result)
		}
	}

	// Sort by score
	sort.Slice(uniqueResults, func(i, j int) bool {
		return uniqueResults[i].Score > uniqueResults[j].Score
	})

	// Return top K results
	if len(uniqueResults) > topK {
		uniqueResults = uniqueResults[:topK]
	}

	return uniqueResults
}

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// Default prompts
const (
	defaultTaskPrompt = `You are an autonomous RAG agent. Your task is to analyze the current state and determine the next action.

Current task: {task}

{retrieved_info}

{reasoning_history}

Based on the current task and the information retrieved so far, decide what to do next:

Options:
1. retrieve - Generate a search query to get more information
2. finish - You have enough information to complete the task

IMPORTANT: Respond in JSON format with the following structure:
{
  "action": "retrieve" or "finish",
  "query": "your search query (required if action is retrieve)",
  "reasoning": "your reasoning for this decision",
  "confidence": 0.0-1.0 (your confidence in this decision)
}

Example responses:
{
  "action": "retrieve",
  "query": "AI trends 2024",
  "reasoning": "Need more specific information about recent AI developments",
  "confidence": 0.85
}

{
  "action": "finish",
  "query": "",
  "reasoning": "Have sufficient information to answer the question comprehensively",
  "confidence": 0.95
}`

	defaultReflectPrompt = `You are an AI assistant reflecting on the retrieval process.

Task: {task}

Retrieved information:
{retrieved_info}

Previous reasoning:
{reasoning_history}

Please reflect on whether you have enough information to complete the task. If not, identify what specific information is missing and what search query would help retrieve it.

IMPORTANT: Respond in JSON format with the following structure:
{
  "has_enough_info": true or false,
  "missing_info": ["list", "of", "missing", "information"],
  "recommended_query": "suggested search query if has_enough_info is false",
  "recommended_action": "what to do next",
  "confidence": 0.0-1.0
}`
)
