// Package multiagent provides the Multi-Agent RAG searcher:
//
//	Parallel SubAgents → ConsensusBuilder → FinalAnswer
package multiagent

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/DotNetAge/gorag/infra/searcher/core"
	"github.com/DotNetAge/gorag/pkg/domain/abstraction"
	"github.com/DotNetAge/gorag/pkg/domain/entity"
	"github.com/DotNetAge/gorag/pkg/logging"
	"github.com/DotNetAge/gorag/pkg/usecase/retrieval"
)

// ---------------------------------------------------------------------------
// SubAgent contracts
// ---------------------------------------------------------------------------

// AgentRole identifies the role an agent plays in the multi-agent collaboration.
type AgentRole string

const (
	RoleResearcher AgentRole = "researcher"
	RoleCritic     AgentRole = "critic"
	RoleWriter     AgentRole = "writer"
)

// AgentResult holds the output produced by a single agent.
type AgentResult struct {
	Role    AgentRole       // role of the agent that produced this result
	Content string          // generated text content
	Chunks  []*entity.Chunk // retrieved chunks that backed the content (may be nil)
}

// SubAgent is a callable participant in the multi-agent pipeline.
// Any searcher can be wrapped via NewSearcherAgentAdapter to become a SubAgent.
type SubAgent interface {
	// Role returns the agent's designated role (e.g. researcher, critic, writer).
	Role() AgentRole
	// Run executes the agent's task given the query and prior agents' results.
	// priorResults may be nil on the first round.
	Run(ctx context.Context, query string, priorResults []*AgentResult) (*AgentResult, error)
}

// Consensus synthesises all agent results into a final answer.
type Consensus interface {
	// Build combines results from all agents into a single coherent answer.
	Build(ctx context.Context, query string, results []*AgentResult) (string, error)
}

// ---------------------------------------------------------------------------
// Searcher
// ---------------------------------------------------------------------------

// Searcher orchestrates a pool of SubAgents for collaborative RAG.
// Agents run concurrently. Failed agents are logged and skipped (non-fatal).
// When a CoordinatorAgent is configured, it decomposes the query into per-agent
// sub-tasks before dispatching; otherwise all agents receive the original query.
type Searcher struct {
	agents      []SubAgent          // registered sub-agents (at least one required)
	consensus   Consensus           // final answer synthesiser (default: writer-first concatenation)
	coordinator CoordinatorAgent    // optional task decomposer; nil = all agents share original query
	logger      logging.Logger      // structured logger
	metrics     abstraction.Metrics // observability metrics collector
}

// Option is a functional option for Searcher.
type Option func(*Searcher)

// WithAgent registers a SubAgent into the pool.
func WithAgent(agent SubAgent) Option {
	return func(s *Searcher) { s.agents = append(s.agents, agent) }
}

// WithConsensus sets a custom consensus builder (default: prefer Writer, else concatenate).
func WithConsensus(c Consensus) Option {
	return func(s *Searcher) { s.consensus = c }
}

// WithCoordinator sets a CoordinatorAgent that decomposes the query into per-agent
// sub-tasks before dispatching. When not set, all agents receive the original query.
func WithCoordinator(c CoordinatorAgent) Option {
	return func(s *Searcher) { s.coordinator = c }
}

// WithLogger sets the logger.
func WithLogger(logger logging.Logger) Option {
	return func(s *Searcher) { s.logger = logger }
}

// WithMetrics sets the metrics collector.
func WithMetrics(m abstraction.Metrics) Option {
	return func(s *Searcher) { s.metrics = m }
}

// New creates a pre-assembled Multi-Agent RAG searcher.
//
// Pipeline: Parallel SubAgents → ConsensusBuildingStep → FinalAnswer
//
// Required: at least one WithAgent.
// Optional: WithConsensus (default concatenation), WithLogger.
//
// Example:
//
//	researcher := multiagent.NewSearcherAgentAdapter(multiagent.RoleResearcher, agenticSearcher)
//	critic := multiagent.NewLLMCriticAgent(evaluator)
//	writer := multiagent.NewSearcherAgentAdapter(multiagent.RoleWriter, nativeSearcher)
//
//	s := multiagent.New(
//	    multiagent.WithAgent(researcher),
//	    multiagent.WithAgent(critic),
//	    multiagent.WithAgent(writer),
//	    multiagent.WithLogger(logger),
//	)
func New(opts ...Option) *Searcher {
	s := &Searcher{
		consensus: &defaultConsensusBuilder{},
		logger:    logging.NewNoopLogger(),
		metrics:   core.DefaultMetrics(),
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// Search runs all agents (optionally with coordinator-assigned sub-queries) in
// parallel, then builds a consensus answer.
func (s *Searcher) Search(ctx context.Context, query string) (string, error) {
	if len(s.agents) == 0 {
		return "", fmt.Errorf("multiagent.Searcher: at least one agent is required")
	}

	start := time.Now()

	s.logger.Info("multi-agent search started", map[string]interface{}{
		"query":       query,
		"agent_count": len(s.agents),
		"coordinated": s.coordinator != nil,
	})

	// Build the per-agent task list, optionally via CoordinatorAgent.
	tasks, err := s.planTasks(ctx, query)
	if err != nil {
		s.metrics.RecordSearchError("multi_agent", err)
		return "", fmt.Errorf("multiagent.Searcher.Search: coordinator failed: %w", err)
	}

	results, err := s.runParallel(ctx, tasks)
	if err != nil {
		s.metrics.RecordSearchError("multi_agent", err)
		return "", fmt.Errorf("multiagent.Searcher.Search: %w", err)
	}

	answer, err := s.consensus.Build(ctx, query, results)
	if err != nil {
		s.metrics.RecordSearchError("multi_agent", err)
		return "", fmt.Errorf("multiagent.Searcher: consensus.Build failed: %w", err)
	}

	s.metrics.RecordSearchDuration("multi_agent", time.Since(start))
	s.metrics.RecordSearchResult("multi_agent", len(results))

	s.logger.Info("multi-agent search completed", map[string]interface{}{
		"query":         query,
		"answer_length": len(answer),
	})
	return answer, nil
}

// planTasks builds the per-agent task slice.
// When a CoordinatorAgent is set it calls Plan() and matches tasks to agents by role.
// Agents that do not appear in the plan receive the original query.
// When no coordinator is set, every agent gets the original query.
func (s *Searcher) planTasks(ctx context.Context, query string) ([]agentTask, error) {
	tasks := make([]agentTask, len(s.agents))
	for i, a := range s.agents {
		tasks[i] = agentTask{agent: a, query: query}
	}

	if s.coordinator == nil {
		return tasks, nil
	}

	plan, err := s.coordinator.Plan(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("CoordinatorAgent.Plan: %w", err)
	}

	// Build role → sub-query lookup.
	roleQuery := make(map[AgentRole]string, len(plan.Tasks))
	for _, t := range plan.Tasks {
		if t.SubQuery != "" {
			roleQuery[t.Role] = t.SubQuery
		}
	}

	// Assign coordinator-provided sub-query to each matched agent; fall back to original.
	for i, a := range s.agents {
		if sq, ok := roleQuery[a.Role()]; ok {
			tasks[i].query = sq
		}
	}

	s.logger.Info("coordinator plan applied", map[string]interface{}{
		"tasks": len(plan.Tasks),
	})
	return tasks, nil
}

// agentTask pairs a SubAgent with the query it should process.
// When no CoordinatorAgent is configured all tasks carry the original query.
type agentTask struct {
	agent SubAgent
	query string
}

// runParallel executes agents in two phases:
//   - Phase 1: all non-Critic agents run concurrently.
//   - Phase 2: Critic agents run with Phase 1 results as priorResults,
//     enabling them to evaluate actual researcher/writer output.
//
// Individual agent failures are logged and skipped; the call fails only if
// every agent in Phase 1 fails (no results for Critic to evaluate).
func (s *Searcher) runParallel(ctx context.Context, tasks []agentTask) ([]*AgentResult, error) {
	type outcome struct {
		result *AgentResult
		err    error
	}

	// Split tasks into non-Critic and Critic groups.
	var phase1Tasks, criticTasks []agentTask
	for _, t := range tasks {
		if t.agent.Role() == RoleCritic {
			criticTasks = append(criticTasks, t)
		} else {
			phase1Tasks = append(phase1Tasks, t)
		}
	}

	// Phase 1: run non-Critic agents concurrently.
	phase1Outcomes := make([]outcome, len(phase1Tasks))
	var wg sync.WaitGroup
	for i, t := range phase1Tasks {
		wg.Add(1)
		go func(idx int, task agentTask) {
			defer wg.Done()
			res, err := task.agent.Run(ctx, task.query, nil)
			phase1Outcomes[idx] = outcome{result: res, err: err}
		}(i, t)
	}
	wg.Wait()

	var phase1Results []*AgentResult
	for _, o := range phase1Outcomes {
		if o.err != nil {
			s.logger.Error("agent execution failed", o.err, map[string]interface{}{})
			continue
		}
		if o.result != nil {
			phase1Results = append(phase1Results, o.result)
		}
	}

	// Phase 2: run Critic agents with phase1 results as priorResults.
	criticOutcomes := make([]outcome, len(criticTasks))
	for i, t := range criticTasks {
		wg.Add(1)
		go func(idx int, task agentTask) {
			defer wg.Done()
			res, err := task.agent.Run(ctx, task.query, phase1Results)
			criticOutcomes[idx] = outcome{result: res, err: err}
		}(i, t)
	}
	wg.Wait()

	var results []*AgentResult
	results = append(results, phase1Results...)
	for _, o := range criticOutcomes {
		if o.err != nil {
			s.logger.Error("critic agent execution failed", o.err, map[string]interface{}{})
			continue
		}
		if o.result != nil {
			results = append(results, o.result)
		}
	}

	if len(results) == 0 {
		return nil, fmt.Errorf("all agents failed to produce results")
	}
	return results, nil
}

// ---------------------------------------------------------------------------
// SearcherAgentAdapter – wraps any Search(ctx, query) implementor as a SubAgent
// ---------------------------------------------------------------------------

// searchable is the minimal interface required by SearcherAgentAdapter.
// Any type that exposes a Search method satisfies it.
type searchable interface {
	Search(ctx context.Context, query string) (string, error)
}

// SearcherAgentAdapter wraps any Searcher so it can participate in a multi-agent pipeline.
type SearcherAgentAdapter struct {
	role     AgentRole  // agent role identifier
	searcher searchable // underlying searcher implementation
}

// NewSearcherAgentAdapter creates a new adapter.
func NewSearcherAgentAdapter(role AgentRole, s searchable) *SearcherAgentAdapter {
	return &SearcherAgentAdapter{role: role, searcher: s}
}

// Role returns the adapter's designated agent role.
func (a *SearcherAgentAdapter) Role() AgentRole { return a.role }

// Run delegates to the underlying searcher's Search method and wraps the result
// as an AgentResult. priorResults are ignored by this adapter.
func (a *SearcherAgentAdapter) Run(ctx context.Context, query string, _ []*AgentResult) (*AgentResult, error) {
	answer, err := a.searcher.Search(ctx, query)
	if err != nil {
		return nil, err
	}
	return &AgentResult{Role: a.role, Content: answer}, nil
}

// ---------------------------------------------------------------------------
// LLMCriticAgent – evaluates researcher output and produces a quality report
// ---------------------------------------------------------------------------

// criticEvaluator is the minimal interface needed by LLMCriticAgent.
type criticEvaluator interface {
	Evaluate(ctx context.Context, query, answer, contextText string) (*retrieval.RAGEScores, error)
}

// LLMCriticAgent evaluates the Researcher's output using a RAG evaluator.
// It implements SubAgent with the RoleCritic role.
type LLMCriticAgent struct {
	evaluator criticEvaluator // RAG evaluation backend
}

// NewLLMCriticAgent creates a new LLM-backed critic agent.
func NewLLMCriticAgent(evaluator criticEvaluator) *LLMCriticAgent {
	return &LLMCriticAgent{evaluator: evaluator}
}

// Role returns RoleCritic.
func (c *LLMCriticAgent) Role() AgentRole { return RoleCritic }

// Run looks for a RoleResearcher result in prior, evaluates it using the RAG
// evaluator, and returns a formatted quality report as the critic's output.
// If no researcher content is found, it returns a no-op result.
func (c *LLMCriticAgent) Run(ctx context.Context, query string, prior []*AgentResult) (*AgentResult, error) {
	var researchContent string
	for _, r := range prior {
		if r.Role == RoleResearcher && r.Content != "" {
			researchContent = r.Content
			break
		}
	}

	if researchContent == "" {
		return &AgentResult{Role: RoleCritic, Content: "No researcher output to evaluate."}, nil
	}

	scores, err := c.evaluator.Evaluate(ctx, query, researchContent, researchContent)
	if err != nil {
		return nil, fmt.Errorf("LLMCriticAgent: Evaluate failed: %w", err)
	}

	feedback := fmt.Sprintf(
		"Critic Report — Faithfulness: %.2f | Relevance: %.2f | Precision: %.2f | Overall: %.2f",
		scores.Faithfulness, scores.AnswerRelevance, scores.ContextPrecision, scores.OverallScore,
	)
	return &AgentResult{Role: RoleCritic, Content: feedback}, nil
}

// ---------------------------------------------------------------------------
// CoordinatorAgent – task decomposition and per-agent query routing
// ---------------------------------------------------------------------------

// AgentTask assigns a refined sub-query to a specific agent role.
// The CoordinatorAgent produces one AgentTask per registered SubAgent.
type AgentTask struct {
	Role     AgentRole // target agent role
	SubQuery string    // refined sub-query for this agent (may equal the original query)
}

// TaskPlan is the output of CoordinatorAgent.Plan.
// It contains one task per sub-agent that should participate in this round.
type TaskPlan struct {
	Tasks []AgentTask
}

// CoordinatorAgent decomposes the original query into per-agent sub-tasks and
// returns a TaskPlan that routes different aspects of the query to the appropriate
// specialist agents (researcher, critic, writer, …).
//
// When no CoordinatorAgent is configured, all agents receive the original query.
type CoordinatorAgent interface {
	// Plan analyses the query and returns a TaskPlan with one entry per agent.
	Plan(ctx context.Context, query string) (*TaskPlan, error)
}

// ---------------------------------------------------------------------------
// defaultConsensusBuilder
// ---------------------------------------------------------------------------

// defaultConsensusBuilder is the built-in Consensus implementation.
// It prefers the Writer agent's output; if absent, it concatenates all contributions.
type defaultConsensusBuilder struct{}

// Build returns the Writer's content when available, or a concatenation of all
// non-empty agent outputs prefixed with their role labels.
func (d *defaultConsensusBuilder) Build(_ context.Context, _ string, results []*AgentResult) (string, error) {
	if len(results) == 0 {
		return "", fmt.Errorf("defaultConsensusBuilder: no agent results to synthesise")
	}
	for _, r := range results {
		if r.Role == RoleWriter && r.Content != "" {
			return r.Content, nil
		}
	}
	var combined string
	for _, r := range results {
		if r.Content != "" {
			combined += fmt.Sprintf("[%s]\n%s\n\n", r.Role, r.Content)
		}
	}
	return combined, nil
}
