package agentic

import (
	"context"
	"fmt"

	chat "github.com/DotNetAge/gochat/pkg/core"
	"github.com/DotNetAge/gochat/pkg/pipeline"
	"github.com/DotNetAge/gorag/pkg/core"
	"github.com/DotNetAge/gorag/pkg/core/agent"
	"github.com/DotNetAge/gorag/pkg/logging"
)

type agenticRetriever struct {
	agent    agent.Agent
	pipeline *pipeline.Pipeline[*core.RetrievalContext]
	logger   logging.Logger
}

// NewRetriever creates a new Agentic RAG retriever.
func NewRetriever(
	a agent.Agent,
	opts ...Option,
) core.Retriever {
	options := defaultOptions()
	for _, opt := range opts {
		opt(options)
	}

	if options.logger == nil {
		options.logger = logging.NewNoopLogger()
	}

	p := pipeline.New[*core.RetrievalContext]()

	// The core of Agentic RAG is the Agent itself which handles reasoning and tool use.
	// We wrap the agent as a pipeline step.
	p.AddStep(&agentStep{
		agent:  a,
		logger: options.logger,
	})

	return &agenticRetriever{
		agent:    a,
		pipeline: p,
		logger:   options.logger,
	}
}

func (r *agenticRetriever) Retrieve(ctx context.Context, queries []string, topK int) ([]*core.RetrievalResult, error) {
	results := make([]*core.RetrievalResult, 0, len(queries))

	for _, q := range queries {
		retrievalCtx := core.NewRetrievalContext(ctx, q)
		
		// If session ID is provided in metadata, we can use it for memory
		sessionID, _ := retrievalCtx.Query.Metadata["session_id"].(string)
		if sessionID != "" {
			retrievalCtx.Agentic.Metadata["session_id"] = sessionID
		}

		if err := r.pipeline.Execute(ctx, retrievalCtx); err != nil {
			return nil, err
		}

		// Flatten any chunks collected during agent execution if any
		var allChunks []*core.Chunk
		for _, group := range retrievalCtx.RetrievedChunks {
			allChunks = append(allChunks, group...)
		}

		res := &core.RetrievalResult{
			Query:  q,
			Chunks: allChunks,
			Answer: retrievalCtx.Answer.Answer,
		}
		
		// Attach agent steps to metadata for traceability
		if steps, ok := retrievalCtx.Agentic.Custom["agent_steps"].([]agent.AgentStep); ok {
			if res.Metadata == nil {
				res.Metadata = make(map[string]any)
			}
			res.Metadata["agent_steps"] = steps
		}

		results = append(results, res)
	}

	return results, nil
}

// agentStep is a pipeline step that delegates execution to an agent.
type agentStep struct {
	agent  agent.Agent
	logger logging.Logger
}

func (s *agentStep) Name() string {
	return "AgentExecution"
}

func (s *agentStep) Execute(ctx context.Context, context *core.RetrievalContext) error {
	s.logger.Debug("starting agent execution", map[string]any{
		"agent": s.agent.Name(),
		"query": context.Query.Text,
	})

	var history []chat.Message
	if context.Agentic != nil {
		history = context.Agentic.History
	}

	resp, err := s.agent.Chat(ctx, context.Query.Text, history)
	if err != nil {
		return fmt.Errorf("agent execution failed: %w", err)
	}

	context.Answer = &core.Result{
		Answer: resp.Response,
	}

	if context.Agentic == nil {
		context.Agentic = core.NewAgenticState()
	}
	context.Agentic.Custom["agent_steps"] = resp.Steps

	return nil
}

// Options for Agentic retriever
type Options struct {
	logger logging.Logger
}

func defaultOptions() *Options {
	return &Options{}
}

type Option func(*Options)

func WithLogger(l logging.Logger) Option {
	return func(o *Options) {
		o.logger = l
	}
}
