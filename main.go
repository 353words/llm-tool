package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/llms/openai"
)

func main() {
	/*
		baseURL := "http://localhost:8080/v1"
		if host := os.Getenv("KRONK_WEB_API_HOST"); host != "" {
			baseURL = host + "/v1"
		}

			llm, err := openai.New(
				openai.WithBaseURL(baseURL),
				openai.WithToken("x"),
				openai.WithModel("Qwen3-8B-Q8_0"),
			)
	*/
	llm, err := openai.New()
	if err != nil {
		log.Fatal(err)
	}

	// Sending initial message to the model, with a list of available tools.
	ctx := context.Background()

	query := `
	Suggest 3 time slots for a 45 minute meeting between Miki & Bill on June 7, 2026.
	Make sure the time slots you sugget don't overlap with existing meetings.
	`

	messageHistory := []llms.MessageContent{
		llms.TextParts(llms.ChatMessageTypeHuman, query),
	}

	resp, err := llm.GenerateContent(ctx, messageHistory, llms.WithTools(availableTools))
	if err != nil {
		log.Fatal(err)
	}
	messageHistory = updateMessageHistory(messageHistory, resp)

	// Execute tool calls requested by the model
	messageHistory, err = executeToolCalls(ctx, llm, messageHistory, resp)
	if err != nil {
		log.Fatal(err)
	}

	// Send query to the model again, this time with a history containing its
	// request to invoke a tool and our response to the tool call.
	fmt.Println("Querying with tool response...")
	resp, err = llm.GenerateContent(ctx, messageHistory, llms.WithTools(availableTools))
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(">>>", resp.Choices[0].Content)
}

// updateMessageHistory updates the message history with the assistant's
// response and requested tool calls.
func updateMessageHistory(messageHistory []llms.MessageContent, resp *llms.ContentResponse) []llms.MessageContent {
	respchoice := resp.Choices[0]

	assistantResponse := llms.TextParts(llms.ChatMessageTypeAI, respchoice.Content)
	for _, tc := range respchoice.ToolCalls {
		assistantResponse.Parts = append(assistantResponse.Parts, tc)
	}
	return append(messageHistory, assistantResponse)
}

// executeToolCalls executes the tool calls in the response and returns the
// updated message history.
func executeToolCalls(ctx context.Context, llm llms.Model, messageHistory []llms.MessageContent, resp *llms.ContentResponse) ([]llms.MessageContent, error) {
	fmt.Println("Executing", len(resp.Choices[0].ToolCalls), "tool calls")
	for _, toolCall := range resp.Choices[0].ToolCalls {
		switch toolCall.FunctionCall.Name {
		case "meetings":
			var args struct {
				User string
				Date string
			}
			if err := json.Unmarshal([]byte(toolCall.FunctionCall.Arguments), &args); err != nil {
				return nil, err
			}

			user := strings.ToLower(args.User)
			date, err := time.Parse("2006-01-02", args.Date)
			if err != nil {
				return nil, err
			}
			meetings := UserMeetings(user, date)

			data, err := json.Marshal(meetings)
			if err != nil {
				return nil, err
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
			messageHistory = append(messageHistory, response)
		default:
			return nil, fmt.Errorf("Unsupported tool: %q", toolCall.FunctionCall.Name)
		}
	}

	return messageHistory, nil
}

// availableTools simulates the tools/functions we're making available for
// the model.
var availableTools = []llms.Tool{
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
