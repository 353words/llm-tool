package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	_ "embed"

	"github.com/ardanlabs/ai-training/foundation/client"
)

//go:embed prompts/system.txt
var systemPrompt string

var tools = []client.D{
	{
		"type": "function",
		"function": client.D{
			"name":        "meetings",
			"description": "Get the meetings (busy time) of a user for a given date. Returns a list of meetings.",
			"parameters": client.D{
				"type": "object",
				"properties": client.D{
					"user": client.D{
						"type":        "string",
						"description": "User name",
					},
					"date": client.D{
						"type":        "string",
						"description": "date in YYYY-MM-DD format",
					},
				},
				"required": []string{"user", "date"},
			},
		},
	},
}

func findMeetings(ctx context.Context, sseClient *client.SSEClient[client.ChatSSE], url string, model string, userPrompt string) (string, error) {
	conversation := []client.D{
		{
			"role":    "system",
			"content": systemPrompt,
		},
		{
			"role":    "user",
			"content": userPrompt,
		},
	}

	for {
		d := client.D{
			"model":       model,
			"messages":    conversation,
			"max_tokens":  32 * 1024,
			"temperature": 0.1,
			"top_p":       0.1,
			"top_k":       50,
			"stream":      true,
			"tools":       tools,
		}

		ch := make(chan client.ChatSSE, 100)
		ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)

		if err := sseClient.Do(ctx, http.MethodPost, url, d, ch); err != nil {
			cancel()
			return "", fmt.Errorf("sse do: %w", err)
		}

		var chunks []string
		var pendingToolCalls []client.ToolCall

		for resp := range ch {
			if len(resp.Choices) == 0 {
				continue
			}

			if len(resp.Choices[0].Delta.ToolCalls) > 0 {
				pendingToolCalls = resp.Choices[0].Delta.ToolCalls
				// Drain remaining events.
				for range ch {
				}
				break
			}

			if resp.Choices[0].Delta.Content != "" {
				chunks = append(chunks, resp.Choices[0].Delta.Content)
			}
		}

		cancel()

		if len(pendingToolCalls) == 0 {
			return strings.Join(chunks, ""), nil
		}

		// Add assistant message with tool calls to conversation.
		argsJSON, _ := json.Marshal(pendingToolCalls[0].Function.Arguments)
		conversation = append(conversation, client.D{
			"role": "assistant",
			"tool_calls": []client.D{
				{
					"id":   pendingToolCalls[0].ID,
					"type": "function",
					"function": client.D{
						"name":      pendingToolCalls[0].Function.Name,
						"arguments": string(argsJSON),
					},
				},
			},
		})

		// Process each tool call.
		for _, toolCall := range pendingToolCalls {
			switch toolCall.Function.Name {
			case "meetings":
				slog.Debug("tool call", "data", toolCall.Function.Arguments)

				user := strings.ToLower(toolCall.Function.Arguments["user"].(string))
				dateStr := toolCall.Function.Arguments["date"].(string)
				date, err := time.Parse("2006-01-02", dateStr)
				if err != nil {
					return "", fmt.Errorf("parse date: %w", err)
				}

				meetings := QueryMeetings(user, date)
				slog.Debug("meetings", "data", meetings)

				data, err := json.Marshal(meetings)
				if err != nil {
					return "", fmt.Errorf("marshal meetings: %w", err)
				}

				conversation = append(conversation, client.D{
					"role":         "tool",
					"tool_call_id": toolCall.ID,
					"content":      string(data),
				})
			default:
				return "", fmt.Errorf("unsupported tool: %q", toolCall.Function.Name)
			}
		}
	}
}

func main() {
	url := "http://localhost:8080/v1/chat/completions"
	if host := os.Getenv("KRONK_WEB_API_HOST"); host != "" {
		url = host + "/v1/chat/completions"
	}

	// model := "Ministral-3-14B-Instruct-2512-Q4_0"
	//model := "Qwen3-8B-Q8_0"
	model := "Ministral-3-8B-Instruct-2512-Q2_K"

	if os.Getenv("DEBUG") != "" {
		h := slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug})
		slog.SetDefault(slog.New(h))
	}

	sseClient := client.NewSSE[client.ChatSSE](client.NoopLogger)

	query := "Suggest 3 time slots for a 45 minute meeting between Miki & Bill on June 7, 2026."
	answer, err := findMeetings(context.Background(), sseClient, url, model, query)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("RESULTS:\n%s\n", answer)
}
