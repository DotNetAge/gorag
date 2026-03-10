package llmutil

import (
	"context"

	gochatcore "github.com/DotNetAge/gochat/pkg/core"
)

// Complete is a helper to adapt gochat's Chat method to the simpler Complete signature
func Complete(ctx context.Context, client gochatcore.Client, prompt string) (string, error) {
	resp, err := client.Chat(ctx, []gochatcore.Message{gochatcore.NewUserMessage(prompt)})
	if err != nil {
		return "", err
	}
	return resp.Content, nil
}

// CompleteStream is a helper to adapt gochat's ChatStream to return a channel of strings
func CompleteStream(ctx context.Context, client gochatcore.Client, prompt string) (<-chan string, error) {
	stream, err := client.ChatStream(ctx, []gochatcore.Message{gochatcore.NewUserMessage(prompt)})
	if err != nil {
		return nil, err
	}

	ch := make(chan string)
	if stream == nil {
		close(ch)
		return ch, nil
	}
	go func() {
		defer stream.Close()
		defer close(ch)
		for stream.Next() {
			ev := stream.Event()
			if ev.Type == gochatcore.EventContent {
				ch <- ev.Content
			}
		}
	}()

	return ch, nil
}
