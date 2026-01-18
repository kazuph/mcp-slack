//go:build integration
// +build integration

package text_test

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/joho/godotenv"
	"github.com/korotovsky/slack-mcp-server/pkg/text"
	"github.com/slack-go/slack"
)

func init() {
	// Load .env file from project root
	godotenv.Load("../../.env")
}

func TestIntegration_RichTextExtraction(t *testing.T) {
	token := os.Getenv("SLACK_MCP_XOXP_TOKEN")
	if token == "" {
		t.Skip("SLACK_MCP_XOXP_TOKEN not set, skipping integration test")
	}

	api := slack.New(token)

	// corp-upsider channel ID (found from list test)
	channelID := "C08RY19PPUN"
	t.Logf("Using corp-upsider channel (ID: %s)", channelID)

	// Get recent messages from the channel
	historyParams := slack.GetConversationHistoryParameters{
		ChannelID: channelID,
		Limit:     10,
	}

	history, err := api.GetConversationHistoryContext(context.Background(), &historyParams)
	if err != nil {
		t.Fatalf("Failed to get conversation history: %v", err)
	}

	t.Logf("Retrieved %d messages from channel", len(history.Messages))

	// Check each message for rich content
	for i, msg := range history.Messages {
		t.Logf("\n--- Message %d ---", i+1)
		t.Logf("Original Text: %q", msg.Text)
		t.Logf("Has Blocks: %v (count: %d)", len(msg.Blocks.BlockSet) > 0, len(msg.Blocks.BlockSet))
		t.Logf("Has Attachments: %v (count: %d)", len(msg.Attachments) > 0, len(msg.Attachments))
		t.Logf("Has Files: %v (count: %d)", len(msg.Files) > 0, len(msg.Files))

		// Log block details
		for j, block := range msg.Blocks.BlockSet {
			t.Logf("  Block %d: Type=%s", j, block.BlockType())
		}

		// Log attachment details
		for j, att := range msg.Attachments {
			t.Logf("  Attachment %d: Title=%q, Text=%q, Fields=%d, NestedBlocks=%d", j, att.Title, truncate(att.Text, 50), len(att.Fields), len(att.Blocks.BlockSet))
			for k, field := range att.Fields {
				t.Logf("    Field %d: %s = %s", k, field.Title, truncate(field.Value, 30))
			}
			for k, block := range att.Blocks.BlockSet {
				t.Logf("    NestedBlock %d: Type=%s", k, block.BlockType())
			}
		}

		// Extract text using our function
		extracted := text.ExtractTextFromMessage(&msg)
		processed := text.ProcessText(extracted)

		t.Logf("Extracted text: %q", truncate(extracted, 200))
		t.Logf("Processed text: %q", truncate(processed, 200))

		// Verify that rich content was extracted
		if len(msg.Attachments) > 0 || len(msg.Blocks.BlockSet) > 0 {
			if extracted == msg.Text {
				t.Logf("WARNING: Extracted text equals original text, rich content may not be extracted")
			} else {
				t.Logf("SUCCESS: Rich content was extracted (text differs from original)")
			}
		}
	}
}

func TestIntegration_ListChannels(t *testing.T) {
	token := os.Getenv("SLACK_MCP_XOXP_TOKEN")
	if token == "" {
		t.Skip("SLACK_MCP_XOXP_TOKEN not set, skipping integration test")
	}

	api := slack.New(token)

	channels, _, err := api.GetConversationsContext(context.Background(), &slack.GetConversationsParameters{
		Types: []string{"public_channel", "private_channel"},
		Limit: 200,
	})
	if err != nil {
		t.Fatalf("Failed to get conversations: %v", err)
	}

	t.Logf("Found %d channels", len(channels))
	for _, ch := range channels {
		t.Logf("Channel: %-30s ID: %s Private: %v", ch.Name, ch.ID, ch.IsPrivate)
	}
}

func truncate(s string, maxLen int) string {
	s = strings.ReplaceAll(s, "\n", "\\n")
	if len(s) > maxLen {
		return s[:maxLen] + "..."
	}
	return s
}
