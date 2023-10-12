package llm

import "context"

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type LlmClient interface {
	ChatComplete(ctx context.Context, messages []*Message) (string, error)
	SingleQuestion(question string) (string, error)

	Close()
}
