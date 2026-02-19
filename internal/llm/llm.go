package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
)

// ExtractedIssue holds a single issue extracted from markdown content.
type ExtractedIssue struct {
	Project     string `json:"project"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Type        string `json:"type"`
	Priority    string `json:"priority"`
	Body        string `json:"body"` // raw source text for this specific issue
}

// Client wraps the Anthropic API for issue extraction.
type Client struct {
	api   *anthropic.Client
	model anthropic.Model
}

// NewClient creates an LLM client with the given API key and model.
func NewClient(apiKey, model string) *Client {
	opts := []option.RequestOption{}
	if apiKey != "" {
		opts = append(opts, option.WithAPIKey(apiKey))
	}
	client := anthropic.NewClient(opts...)
	return &Client{
		api:   &client,
		model: anthropic.Model(model),
	}
}

// buildPrompt constructs the system and user prompts for issue extraction.
func buildPrompt(content string, projects []string) (system string, user string) {
	system = `You extract structured issues from markdown content. Return ONLY a JSON array of objects with these fields:
- "project": the project name these issues belong to (infer from headings like "## Project <name>" or context)
- "title": concise issue title
- "description": brief description of the issue (can be empty string if the title is self-explanatory)
- "type": one of "feature", "bug", "chore"
- "priority": one of "low", "medium", "high"
- "body": the exact original source text from the input that relates to this specific issue (preserve formatting, include any sub-bullets, details, or context lines that belong to this issue)

Rules:
- Each numbered/bulleted item is one issue
- Infer type from context (new capabilities = feature, problems = bug, maintenance = chore)
- Default priority to "medium" unless context suggests otherwise
- Match project names to the known projects list when possible
- The "body" field must contain only the relevant portion of the original text for that issue, not the entire document
- For sub-issues (e.g., "1.1 Sub-task", "2.1 Detail"), include the parent issue's text in the body field before the sub-issue text, as the parent may provide additional context. For example, if "1. Authentication system" has sub-issue "1.1 Add login form", the sub-issue body should be "1. Authentication system\n1.1 Add login form"
- If a project section contains no issues, do NOT generate any entries for that project. Never create placeholder issues like "no issues specified" or "N/A"
- Return valid JSON only, no markdown fencing or explanation`

	var sb strings.Builder
	if len(projects) > 0 {
		sb.WriteString("Known projects: ")
		sb.WriteString(strings.Join(projects, ", "))
		sb.WriteString("\n\n")
	}
	sb.WriteString("Extract issues from this markdown:\n\n")
	sb.WriteString(content)
	user = sb.String()
	return
}

// ExtractIssues sends markdown content to the LLM and returns structured issues.
func (c *Client) ExtractIssues(ctx context.Context, content string, projects []string) ([]ExtractedIssue, error) {
	systemPrompt, userPrompt := buildPrompt(content, projects)

	msg, err := c.api.Messages.New(ctx, anthropic.MessageNewParams{
		Model:     c.model,
		MaxTokens: 4096,
		System: []anthropic.TextBlockParam{
			{Text: systemPrompt},
		},
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock(userPrompt)),
		},
	})
	if err != nil {
		return nil, fmt.Errorf("anthropic API call: %w", err)
	}

	// Extract text from response
	var text string
	for _, block := range msg.Content {
		if block.Type == "text" {
			text = block.Text
			break
		}
	}

	if text == "" {
		return nil, fmt.Errorf("no text content in API response")
	}

	// Strip markdown fencing if present
	text = strings.TrimSpace(text)
	if strings.HasPrefix(text, "```") {
		lines := strings.SplitN(text, "\n", 2)
		if len(lines) > 1 {
			text = lines[1]
		}
		if idx := strings.LastIndex(text, "```"); idx >= 0 {
			text = text[:idx]
		}
		text = strings.TrimSpace(text)
	}

	var issues []ExtractedIssue
	if err := json.Unmarshal([]byte(text), &issues); err != nil {
		return nil, fmt.Errorf("parse LLM response as JSON: %w\nraw response: %s", err, text)
	}

	return issues, nil
}
