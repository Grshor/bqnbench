// Package llm is a minimal OpenAI-compatible chat completions client.
// Works with llama.cpp servers, OpenAI, and any /v1/chat/completions API.
package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"
)

// Model describes one chat endpoint.
type Model struct {
	Name     string `json:"name"`        // display name for the scoreboard
	BaseURL  string `json:"base_url"`    // e.g. http://127.0.0.1:8091/v1
	APIKey   string `json:"api_key"`     // may be empty for local servers
	APIKeyEv string `json:"api_key_env"` // if set, key is read from this env var
	Model    string `json:"model"`       // model identifier sent to the API
}

func (m Model) key() string {
	if m.APIKeyEv != "" {
		return os.Getenv(m.APIKeyEv)
	}
	return m.APIKey
}

// Message is one chat message.
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatRequest struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	Temperature float64   `json:"temperature"`
	MaxTokens   int       `json:"max_tokens,omitempty"`
}

type chatResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

// Chat sends the messages and returns the assistant reply.
func Chat(ctx context.Context, m Model, messages []Message, temperature float64, maxTokens int) (string, error) {
	body, err := json.Marshal(chatRequest{
		Model:       m.Model,
		Messages:    messages,
		Temperature: temperature,
		MaxTokens:   maxTokens,
	})
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}
	ctx, cancel := context.WithTimeout(ctx, 120*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		strings.TrimSuffix(m.BaseURL, "/")+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	if k := m.key(); k != "" {
		req.Header.Set("Authorization", "Bearer "+k)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("request: %w", err)
	}
	defer resp.Body.Close()
	var cr chatResponse
	if err := json.NewDecoder(resp.Body).Decode(&cr); err != nil {
		return "", fmt.Errorf("decode (http %d): %w", resp.StatusCode, err)
	}
	if cr.Error != nil {
		return "", fmt.Errorf("api error: %s", cr.Error.Message)
	}
	if len(cr.Choices) == 0 {
		return "", fmt.Errorf("api returned no choices (http %d)", resp.StatusCode)
	}
	return cr.Choices[0].Message.Content, nil
}
