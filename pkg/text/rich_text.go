package text

import (
	"fmt"
	"strings"

	"github.com/slack-go/slack"
)

// ExtractTextFromMessage extracts all text content from a Slack message,
// including text from blocks and attachments (rich format support).
func ExtractTextFromMessage(msg *slack.Message) string {
	var parts []string

	// 1. Basic text field
	if msg.Text != "" {
		parts = append(parts, msg.Text)
	}

	// 2. Extract text from Blocks
	if len(msg.Blocks.BlockSet) > 0 {
		blockText := extractTextFromBlocks(msg.Blocks.BlockSet)
		if blockText != "" {
			parts = append(parts, blockText)
		}
	}

	// 3. Extract text from Attachments
	if len(msg.Attachments) > 0 {
		attachmentText := extractTextFromAttachments(msg.Attachments)
		if attachmentText != "" {
			parts = append(parts, attachmentText)
		}
	}

	// 4. Extract text from Files (file descriptions)
	if len(msg.Files) > 0 {
		fileText := extractTextFromFiles(msg.Files)
		if fileText != "" {
			parts = append(parts, fileText)
		}
	}

	// Deduplicate and join
	return deduplicateAndJoin(parts)
}

// extractTextFromBlocks extracts text from various block types
func extractTextFromBlocks(blocks []slack.Block) string {
	var parts []string

	for _, block := range blocks {
		switch b := block.(type) {
		case *slack.SectionBlock:
			parts = append(parts, extractTextFromSectionBlock(b)...)
		case *slack.RichTextBlock:
			parts = append(parts, extractTextFromRichTextBlock(b)...)
		case *slack.HeaderBlock:
			if b.Text != nil {
				parts = append(parts, b.Text.Text)
			}
		case *slack.ContextBlock:
			parts = append(parts, extractTextFromContextBlock(b)...)
		}
	}

	return strings.Join(parts, "\n")
}

// extractTextFromSectionBlock extracts text from a section block
func extractTextFromSectionBlock(block *slack.SectionBlock) []string {
	var parts []string

	if block.Text != nil {
		parts = append(parts, block.Text.Text)
	}

	for _, field := range block.Fields {
		if field != nil {
			parts = append(parts, field.Text)
		}
	}

	return parts
}

// extractTextFromRichTextBlock extracts text from a rich text block
func extractTextFromRichTextBlock(block *slack.RichTextBlock) []string {
	var parts []string

	for _, element := range block.Elements {
		parts = append(parts, extractTextFromRichTextElement(element)...)
	}

	return parts
}

// extractTextFromRichTextElement extracts text from rich text elements
func extractTextFromRichTextElement(element slack.RichTextElement) []string {
	var parts []string

	switch e := element.(type) {
	case *slack.RichTextSection:
		for _, elem := range e.Elements {
			parts = append(parts, extractTextFromRichTextSectionElement(elem)...)
		}
	case *slack.RichTextList:
		for _, item := range e.Elements {
			parts = append(parts, extractTextFromRichTextElement(item)...)
		}
	case *slack.RichTextQuote:
		for _, elem := range e.Elements {
			parts = append(parts, extractTextFromRichTextSectionElement(elem)...)
		}
	case *slack.RichTextPreformatted:
		for _, elem := range e.Elements {
			parts = append(parts, extractTextFromRichTextSectionElement(elem)...)
		}
	}

	return parts
}

// extractTextFromRichTextSectionElement extracts text from section elements
func extractTextFromRichTextSectionElement(element slack.RichTextSectionElement) []string {
	var parts []string

	switch e := element.(type) {
	case *slack.RichTextSectionTextElement:
		if e.Text != "" {
			parts = append(parts, e.Text)
		}
	case *slack.RichTextSectionLinkElement:
		// Include both URL and link text
		linkText := e.Text
		if linkText == "" {
			linkText = e.URL
		}
		if e.URL != "" {
			parts = append(parts, e.URL+" - "+linkText)
		}
	case *slack.RichTextSectionUserElement:
		parts = append(parts, "<@"+e.UserID+">")
	case *slack.RichTextSectionChannelElement:
		parts = append(parts, "<#"+e.ChannelID+">")
	case *slack.RichTextSectionEmojiElement:
		parts = append(parts, ":"+e.Name+":")
	case *slack.RichTextSectionDateElement:
		parts = append(parts, fmt.Sprintf("%d", e.Timestamp))
	}

	return parts
}

// extractTextFromContextBlock extracts text from context blocks
func extractTextFromContextBlock(block *slack.ContextBlock) []string {
	var parts []string

	for _, element := range block.ContextElements.Elements {
		switch e := element.(type) {
		case *slack.TextBlockObject:
			parts = append(parts, e.Text)
		case *slack.ImageBlockElement:
			if e.AltText != "" {
				parts = append(parts, "[Image: "+e.AltText+"]")
			}
		}
	}

	return parts
}

// extractTextFromAttachments extracts text from message attachments
func extractTextFromAttachments(attachments []slack.Attachment) string {
	var parts []string

	for _, att := range attachments {
		// Title
		if att.Title != "" {
			titleText := att.Title
			if att.TitleLink != "" {
				titleText += " (" + att.TitleLink + ")"
			}
			parts = append(parts, titleText)
		}

		// Pretext
		if att.Pretext != "" {
			parts = append(parts, att.Pretext)
		}

		// Main text
		if att.Text != "" {
			parts = append(parts, att.Text)
		}

		// Author
		if att.AuthorName != "" {
			parts = append(parts, "Author: "+att.AuthorName)
		}

		// Fields
		for _, field := range att.Fields {
			fieldText := field.Title + ": " + field.Value
			parts = append(parts, fieldText)
		}

		// Footer
		if att.Footer != "" {
			parts = append(parts, att.Footer)
		}

		// Nested blocks in attachment
		if len(att.Blocks.BlockSet) > 0 {
			blockText := extractTextFromBlocks(att.Blocks.BlockSet)
			if blockText != "" {
				parts = append(parts, blockText)
			}
		}
	}

	return strings.Join(parts, "\n")
}

// extractTextFromFiles extracts text from file metadata
func extractTextFromFiles(files []slack.File) string {
	var parts []string

	for _, file := range files {
		fileInfo := "[File: " + file.Name + "]"
		if file.Title != "" && file.Title != file.Name {
			fileInfo = "[File: " + file.Title + " (" + file.Name + ")]"
		}
		parts = append(parts, fileInfo)

		// Include file preview text if available
		if file.PreviewHighlight != "" {
			parts = append(parts, file.PreviewHighlight)
		}
	}

	return strings.Join(parts, "\n")
}

// ExtractTextFromSearchMessage extracts all text content from a Slack SearchMessage,
// including text from blocks and attachments (rich format support).
func ExtractTextFromSearchMessage(msg *slack.SearchMessage) string {
	var parts []string

	// 1. Basic text field
	if msg.Text != "" {
		parts = append(parts, msg.Text)
	}

	// 2. Extract text from Blocks
	if len(msg.Blocks.BlockSet) > 0 {
		blockText := extractTextFromBlocks(msg.Blocks.BlockSet)
		if blockText != "" {
			parts = append(parts, blockText)
		}
	}

	// 3. Extract text from Attachments
	if len(msg.Attachments) > 0 {
		attachmentText := extractTextFromAttachments(msg.Attachments)
		if attachmentText != "" {
			parts = append(parts, attachmentText)
		}
	}

	// Deduplicate and join
	return deduplicateAndJoin(parts)
}

// deduplicateAndJoin removes duplicate content and joins with newlines
func deduplicateAndJoin(parts []string) string {
	if len(parts) == 0 {
		return ""
	}

	// If first part (text) is contained in subsequent parts (blocks), skip it
	// This handles cases where text is duplicated in blocks
	seen := make(map[string]bool)
	var unique []string

	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if !seen[part] {
			seen[part] = true
			unique = append(unique, part)
		}
	}

	return strings.Join(unique, "\n")
}
