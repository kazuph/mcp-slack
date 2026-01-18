package text

import (
	"testing"

	"github.com/slack-go/slack"
)

func TestExtractTextFromMessage_BasicText(t *testing.T) {
	msg := &slack.Message{
		Msg: slack.Msg{
			Text: "Hello, world!",
		},
	}

	result := ExtractTextFromMessage(msg)
	if result != "Hello, world!" {
		t.Errorf("Expected 'Hello, world!', got '%s'", result)
	}
}

func TestExtractTextFromMessage_WithAttachments(t *testing.T) {
	msg := &slack.Message{
		Msg: slack.Msg{
			Text: "Main message",
			Attachments: []slack.Attachment{
				{
					Title:   "Attachment Title",
					Text:    "Attachment content",
					Pretext: "Pretext here",
					Fields: []slack.AttachmentField{
						{Title: "Field1", Value: "Value1"},
						{Title: "Field2", Value: "Value2"},
					},
				},
			},
		},
	}

	result := ExtractTextFromMessage(msg)

	// Check that all parts are present
	expected := []string{
		"Main message",
		"Attachment Title",
		"Pretext here",
		"Attachment content",
		"Field1: Value1",
		"Field2: Value2",
	}

	for _, exp := range expected {
		if !contains(result, exp) {
			t.Errorf("Expected result to contain '%s', got: %s", exp, result)
		}
	}
}

func TestExtractTextFromMessage_WithSectionBlock(t *testing.T) {
	msg := &slack.Message{
		Msg: slack.Msg{
			Blocks: slack.Blocks{
				BlockSet: []slack.Block{
					&slack.SectionBlock{
						Type: slack.MBTSection,
						Text: &slack.TextBlockObject{
							Type: "mrkdwn",
							Text: "Section text content",
						},
					},
				},
			},
		},
	}

	result := ExtractTextFromMessage(msg)
	if !contains(result, "Section text content") {
		t.Errorf("Expected result to contain 'Section text content', got: %s", result)
	}
}

func TestExtractTextFromMessage_WithRichTextBlock(t *testing.T) {
	msg := &slack.Message{
		Msg: slack.Msg{
			Blocks: slack.Blocks{
				BlockSet: []slack.Block{
					&slack.RichTextBlock{
						Type:    slack.MBTRichText,
						BlockID: "block1",
						Elements: []slack.RichTextElement{
							&slack.RichTextSection{
								Type: slack.RTESection,
								Elements: []slack.RichTextSectionElement{
									&slack.RichTextSectionTextElement{
										Type: slack.RTSEText,
										Text: "Rich text content",
									},
								},
							},
						},
					},
				},
			},
		},
	}

	result := ExtractTextFromMessage(msg)
	if !contains(result, "Rich text content") {
		t.Errorf("Expected result to contain 'Rich text content', got: %s", result)
	}
}

func TestExtractTextFromMessage_WithFiles(t *testing.T) {
	msg := &slack.Message{
		Msg: slack.Msg{
			Text: "Check this file",
			Files: []slack.File{
				{
					Name:  "document.pdf",
					Title: "Important Document",
				},
			},
		},
	}

	result := ExtractTextFromMessage(msg)
	if !contains(result, "Check this file") {
		t.Errorf("Expected result to contain 'Check this file', got: %s", result)
	}
	if !contains(result, "[File: Important Document (document.pdf)]") {
		t.Errorf("Expected result to contain file info, got: %s", result)
	}
}

func TestExtractTextFromMessage_Deduplication(t *testing.T) {
	msg := &slack.Message{
		Msg: slack.Msg{
			Text: "Same text",
			Blocks: slack.Blocks{
				BlockSet: []slack.Block{
					&slack.SectionBlock{
						Type: slack.MBTSection,
						Text: &slack.TextBlockObject{
							Type: "plain_text",
							Text: "Same text",
						},
					},
				},
			},
		},
	}

	result := ExtractTextFromMessage(msg)
	// Should only appear once due to deduplication
	count := 0
	for i := 0; i < len(result); i++ {
		if i+len("Same text") <= len(result) && result[i:i+len("Same text")] == "Same text" {
			count++
		}
	}
	if count > 1 {
		t.Errorf("Expected 'Same text' to appear only once, appeared %d times in: %s", count, result)
	}
}

func TestExtractTextFromMessage_ComplexAttachment(t *testing.T) {
	// Simulates a typical expense management bot attachment
	msg := &slack.Message{
		Msg: slack.Msg{
			Attachments: []slack.Attachment{
				{
					Title:      "経費精算のお知らせ",
					TitleLink:  "https://example.com/expense/123",
					AuthorName: "Expense Bot",
					Text:       "新しい経費が登録されました",
					Fields: []slack.AttachmentField{
						{Title: "金額", Value: "¥10,000"},
						{Title: "カテゴリ", Value: "交通費"},
						{Title: "日付", Value: "2025-01-17"},
					},
					Footer: "Expense Manager",
				},
			},
		},
	}

	result := ExtractTextFromMessage(msg)

	expected := []string{
		"経費精算のお知らせ",
		"https://example.com/expense/123",
		"Expense Bot",
		"新しい経費が登録されました",
		"金額: ¥10,000",
		"カテゴリ: 交通費",
		"日付: 2025-01-17",
		"Expense Manager",
	}

	for _, exp := range expected {
		if !contains(result, exp) {
			t.Errorf("Expected result to contain '%s', got: %s", exp, result)
		}
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
