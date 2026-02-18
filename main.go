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
	"github.com/tmc/langchaingo/llms/openai"
)

var systemPrompt = `
You're a helpful scheduling assistant.

When asked to schedule a meeting between multiple people:
1. Use the "meetings" tool to get the existing meetings for EACH participant separately
2. Identify ALL blocked time spans from ALL participants
3. Propose time slots that work for EVERYONE - the proposed times must NOT overlap with ANY participant's existing meetings
4. Ensure the proposed time slots have enough duration for the requested meeting length

Return exactly and only the proposed time ranges as HH:MM-HH:MM, one per line, with no extra text.
`

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

func callQueryMeetings(arguments string) (string, error) {
	var args struct {
		User string
		Date string
	}
	if err := json.Unmarshal([]byte(arguments), &args); err != nil {
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

	return string(data), nil
}

func findMeetings(ctx context.Context, llm llms.Model, userPrompt string) (string, error) {
	messages := []llms.MessageContent{
		llms.TextParts(llms.ChatMessageTypeSystem, systemPrompt),
		llms.TextParts(llms.ChatMessageTypeHuman, userPrompt),
	}

	// Loop until no more function calls
	for {
		resp, err := llm.GenerateContent(ctx, messages, llms.WithTools(Tools))
		if err != nil {
			return "", err
		}

		respchoice := resp.Choices[0]
		if len(respchoice.ToolCalls) == 0 {
			return respchoice.Content, nil
		}

		assistantResponse := llms.TextParts(llms.ChatMessageTypeAI, respchoice.Content)
		for _, tc := range respchoice.ToolCalls {
			assistantResponse.Parts = append(assistantResponse.Parts, tc)
		}
		messages = append(messages, assistantResponse)

		for _, toolCall := range respchoice.ToolCalls {
			switch toolCall.FunctionCall.Name {
			case "meetings":
				slog.Debug("tool call", "data", toolCall.FunctionCall.Arguments)
				data, err := callQueryMeetings(toolCall.FunctionCall.Arguments)
				if err != nil {
					return "", err
				}

				response := llms.MessageContent{
					Role: llms.ChatMessageTypeTool,
					Parts: []llms.ContentPart{
						llms.ToolCallResponse{
							ToolCallID: toolCall.ID,
							Name:       toolCall.FunctionCall.Name,
							Content:    data,
						},
					},
				}
				messages = append(messages, response)
			default:
				return "", fmt.Errorf("unsupported tool: %q", toolCall.FunctionCall.Name)
			}
		}
	}
}

func main() {
	baseURL := "http://localhost:8080/v1"
	if host := os.Getenv("KRONK_WEB_API_HOST"); host != "" {
		baseURL = host + "/v1"
	}

	llm, err := openai.New(
		openai.WithBaseURL(baseURL),
		openai.WithToken("x"),
		openai.WithModel("Ministral-3-3B-Instruct-2512-Q6_K"),
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

	query := "Suggest time slots for a 45 minute meeting between Miki & Bill on June 7, 2026."
	answer, err := findMeetings(context.Background(), llm, query)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	fmt.Println(answer)
}
