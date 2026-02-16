package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	_ "embed"

	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/llms/googleai"
)

//go:embed prompts/system.txt
var systemPrompt string

var Tools = []llms.Tool{
	{
		Type: "function",
		Function: &llms.FunctionDefinition{
			Name:        "meetings",
			Description: "Get the meetings (busy time) of a user for a given date. Returns a list of meetings.",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"user": map[string]any{
						"type":        "string",
						"description": "User name",
					},
					"date": map[string]any{
						"type":        "string",
						"description": "date in YYYY-MM-DD format",
					},
				},
				"required": []string{"user", "date"},
			},
		},
	},
}

func findMeetings(ctx context.Context, llm llms.Model, userPrompt string) (string, error) {
	messages := []llms.MessageContent{
		llms.TextParts(llms.ChatMessageTypeSystem, systemPrompt),
		llms.TextParts(llms.ChatMessageTypeHuman, userPrompt),
	}

	resp, err := llm.GenerateContent(ctx, messages, llms.WithTools(Tools))
	if err != nil {
		return "", err
	}

	respchoice := resp.Choices[0]
	assistantResponse := llms.TextParts(llms.ChatMessageTypeAI, respchoice.Content)
	for _, tc := range respchoice.ToolCalls {
		assistantResponse.Parts = append(assistantResponse.Parts, tc)
	}
	messages = append(messages, assistantResponse)

	for _, toolCall := range respchoice.ToolCalls {
		switch toolCall.FunctionCall.Name {
		case "meetings":
			slog.Debug("tool call", "data", toolCall.FunctionCall.Arguments)

			var args struct {
				User string
				Date string
			}
			if err := json.Unmarshal([]byte(toolCall.FunctionCall.Arguments), &args); err != nil {
				return "", err
			}

			user := strings.ToLower(args.User)
			date, err := time.Parse("2006-01-02", args.Date)
			if err != nil {
				return "", err
			}

			meetings := QueryMeetings(user, date)
			slog.Debug("meetings", "data", meetings)

			data, err := json.Marshal(meetings)
			if err != nil {
				return "", err
			}

			response := llms.MessageContent{
				Role: llms.ChatMessageTypeTool,
				Parts: []llms.ContentPart{
					llms.ToolCallResponse{
						ToolCallID: toolCall.ID,
						Name:       toolCall.FunctionCall.Name,
						Content:    string(data),
					},
				},
			}
			messages = append(messages, response)
		default:
			return "", fmt.Errorf("unsupported tool: %q", toolCall.FunctionCall.Name)
		}
	}

	resp, err = llm.GenerateContent(ctx, messages, llms.WithTools(Tools))
	if err != nil {
		return "", err
	}

	return resp.Choices[0].Content, nil
}

func main() {
	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		fmt.Fprintf(os.Stderr, "error: GEMINI_API_KEY environment variable not set\n")
		os.Exit(1)
	}

	llm, err := googleai.New(
		context.Background(),
		googleai.WithAPIKey(apiKey),
		//		googleai.WithDefaultModel("gemini-1.5-flash"),
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	if os.Getenv("DEBUG") != "" {
		h := slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug})
		log := slog.New(h)
		slog.SetDefault(log)
	}

	query := "Suggest 3 time slots for a 45 minute meeting between Miki & Bill on June 7, 2026."
	answer, err := findMeetings(context.Background(), llm, query)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("RESULTS:\n%s\n", answer)
}
