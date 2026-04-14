package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/cialloclaw/cialloclaw/services/local-service/internal/model"
)

const lsServiceProvider = "ls_service_chat_completions"

type chatCompletionsClient struct {
	endpoint   string
	modelID    string
	apiKey     string
	httpClient *http.Client
}

type chatCompletionsRequest struct {
	Model    string                   `json:"model"`
	Messages []chatCompletionsMessage `json:"messages"`
}

type chatCompletionsMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatCompletionsResponse struct {
	ID      string `json:"id"`
	Choices []struct {
		Message struct {
			Content          json.RawMessage `json:"content"`
			ReasoningContent json.RawMessage `json:"reasoning_content"`
		} `json:"message"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
}

func newChatCompletionsClient(cfg externalModelConfig) (*chatCompletionsClient, error) {
	if strings.TrimSpace(cfg.APIKey) == "" {
		return nil, fmt.Errorf("LS-service api key is required")
	}
	if strings.TrimSpace(cfg.BaseURL) == "" {
		return nil, fmt.Errorf("LS-service base_url is required")
	}
	if strings.TrimSpace(cfg.Model) == "" {
		return nil, fmt.Errorf("LS-service model is required")
	}

	return &chatCompletionsClient{
		endpoint: strings.TrimSpace(cfg.BaseURL),
		modelID:  strings.TrimSpace(cfg.Model),
		apiKey:   strings.TrimSpace(cfg.APIKey),
		httpClient: &http.Client{
			Timeout: 120 * time.Second,
			Transport: &http.Transport{
				Proxy:                 http.ProxyFromEnvironment,
				ForceAttemptHTTP2:     false,
				MaxIdleConns:          16,
				IdleConnTimeout:       30 * time.Second,
				TLSHandshakeTimeout:   15 * time.Second,
				ExpectContinueTimeout: 1 * time.Second,
			},
		},
	}, nil
}

func (c *chatCompletionsClient) GenerateText(ctx context.Context, request model.GenerateTextRequest) (model.GenerateTextResponse, error) {
	input := strings.TrimSpace(request.Input)
	if input == "" {
		return model.GenerateTextResponse{}, fmt.Errorf("generate text input is required")
	}

	body, err := json.Marshal(chatCompletionsRequest{
		Model: c.modelID,
		Messages: []chatCompletionsMessage{{
			Role:    "user",
			Content: input,
		}},
	})
	if err != nil {
		return model.GenerateTextResponse{}, fmt.Errorf("marshal chat completions request: %w", err)
	}

	start := time.Now()
	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodPost, c.chatCompletionsURL(), bytes.NewReader(body))
	if err != nil {
		return model.GenerateTextResponse{}, fmt.Errorf("build chat completions request: %w", err)
	}
	httpRequest.Header.Set("Content-Type", "application/json")
	httpRequest.Header.Set("Accept", "application/json")
	httpRequest.Header.Set("Authorization", "Bearer "+c.apiKey)
	httpRequest.Header.Set("User-Agent", "CialloClaw-LS-service/1.0")

	response, err := c.httpClient.Do(httpRequest)
	if err != nil {
		log.Printf("LS-service chat completions request_failed model=%s url=%s error=%v", c.modelID, c.chatCompletionsURL(), err)
		return model.GenerateTextResponse{}, fmt.Errorf("chat completions request failed: %w", err)
	}
	defer response.Body.Close()

	latencyMS := time.Since(start).Milliseconds()
	responseBody, err := io.ReadAll(response.Body)
	if err != nil {
		log.Printf("LS-service chat completions read_failed model=%s url=%s status=%d error=%v", c.modelID, c.chatCompletionsURL(), response.StatusCode, err)
		return model.GenerateTextResponse{}, fmt.Errorf("read chat completions response: %w", err)
	}

	log.Printf("LS-service chat completions response model=%s url=%s status=%d latency_ms=%d", c.modelID, c.chatCompletionsURL(), response.StatusCode, latencyMS)

	if response.StatusCode != http.StatusOK {
		parsedError := parseErrorBody(responseBody)
		log.Printf("LS-service chat completions upstream_error model=%s url=%s status=%d error=%s", c.modelID, c.chatCompletionsURL(), response.StatusCode, parsedError)
		return model.GenerateTextResponse{}, fmt.Errorf("chat completions status %d: %s", response.StatusCode, parsedError)
	}

	var payload chatCompletionsResponse
	if err := json.Unmarshal(responseBody, &payload); err != nil {
		log.Printf("LS-service chat completions decode_failed model=%s url=%s status=%d error=%v body=%s", c.modelID, c.chatCompletionsURL(), response.StatusCode, err, truncateForError(strings.TrimSpace(string(responseBody)), 240))
		return model.GenerateTextResponse{}, fmt.Errorf("decode chat completions response: %w", err)
	}
	if len(payload.Choices) == 0 {
		log.Printf("LS-service chat completions empty_choices model=%s url=%s status=%d request_id=%s body=%s", c.modelID, c.chatCompletionsURL(), response.StatusCode, strings.TrimSpace(payload.ID), truncateForError(strings.TrimSpace(string(responseBody)), 240))
		return model.GenerateTextResponse{}, fmt.Errorf("chat completions returned no choices")
	}

	outputText := extractMessageText(payload.Choices[0].Message.Content)
	if outputText == "" {
		outputText = extractMessageText(payload.Choices[0].Message.ReasoningContent)
	}
	if outputText == "" {
		log.Printf("LS-service chat completions empty_content model=%s url=%s status=%d request_id=%s body=%s", c.modelID, c.chatCompletionsURL(), response.StatusCode, strings.TrimSpace(payload.ID), truncateForError(strings.TrimSpace(string(responseBody)), 240))
		return model.GenerateTextResponse{}, fmt.Errorf("chat completions returned empty content")
	}

	log.Printf(
		"LS-service chat completions success model=%s url=%s status=%d request_id=%s total_tokens=%d",
		c.modelID,
		c.chatCompletionsURL(),
		response.StatusCode,
		strings.TrimSpace(payload.ID),
		payload.Usage.TotalTokens,
	)

	return model.GenerateTextResponse{
		TaskID:     request.TaskID,
		RunID:      request.RunID,
		RequestID:  strings.TrimSpace(payload.ID),
		Provider:   lsServiceProvider,
		ModelID:    c.modelID,
		OutputText: outputText,
		Usage: model.TokenUsage{
			InputTokens:  payload.Usage.PromptTokens,
			OutputTokens: payload.Usage.CompletionTokens,
			TotalTokens:  payload.Usage.TotalTokens,
		},
		LatencyMS: latencyMS,
	}, nil
}

func (c *chatCompletionsClient) chatCompletionsURL() string {
	trimmed := strings.TrimRight(strings.TrimSpace(c.endpoint), "/")
	if strings.HasSuffix(trimmed, "/chat/completions") {
		return trimmed
	}
	return trimmed + "/chat/completions"
}

func parseErrorBody(body []byte) string {
	trimmed := strings.TrimSpace(string(body))
	if trimmed == "" {
		return "empty response body"
	}

	var payload struct {
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &payload); err == nil {
		if message := strings.TrimSpace(payload.Error.Message); message != "" {
			return message
		}
	}

	return truncateForError(trimmed, 240)
}

func extractMessageText(raw json.RawMessage) string {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		return ""
	}

	var stringValue string
	if err := json.Unmarshal(trimmed, &stringValue); err == nil {
		return strings.TrimSpace(stringValue)
	}

	var segments []struct {
		Text string `json:"text"`
	}
	if err := json.Unmarshal(trimmed, &segments); err == nil {
		parts := make([]string, 0, len(segments))
		for _, segment := range segments {
			if text := strings.TrimSpace(segment.Text); text != "" {
				parts = append(parts, text)
			}
		}
		return strings.Join(parts, "\n")
	}

	var single struct {
		Text    string `json:"text"`
		Content string `json:"content"`
	}
	if err := json.Unmarshal(trimmed, &single); err == nil {
		if text := strings.TrimSpace(single.Text); text != "" {
			return text
		}
		return strings.TrimSpace(single.Content)
	}

	return ""
}

func truncateForError(value string, max int) string {
	if len(value) <= max {
		return value
	}
	if max <= 3 {
		return value[:max]
	}
	return value[:max-3] + "..."
}
