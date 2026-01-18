package handler

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"unicode"

	"github.com/gocarina/gocsv"
	"github.com/korotovsky/slack-mcp-server/pkg/provider"
	"github.com/mark3labs/mcp-go/mcp"
)

type UserResolution struct {
	UserID      string `json:"userID"`
	UserName    string `json:"userName"`
	RealName    string `json:"realName"`
	DisplayName string `json:"displayName"`
	Email       string `json:"email"`
	MatchType   string `json:"matchType"`
	IsBot       bool   `json:"isBot"`
}

type UsersHandler struct {
	apiProvider *provider.ApiProvider
}

func NewUsersHandler(apiProvider *provider.ApiProvider) *UsersHandler {
	return &UsersHandler{
		apiProvider: apiProvider,
	}
}

// normalizeString removes invisible characters (zero-width spaces, etc.)
func normalizeString(s string) string {
	return strings.Map(func(r rune) rune {
		// Remove common invisible characters
		if r == '\u200B' || r == '\u200C' || r == '\u200D' || r == '\uFEFF' || unicode.IsControl(r) {
			return -1 // Remove the character
		}
		return r
	}, s)
}

func (uh *UsersHandler) UsersResolveHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// Parse parameters
	query := request.GetString("query", "")
	if query == "" {
		return nil, errors.New("query must be a non-empty string")
	}

	searchType := request.GetString("search_type", "auto")

	// Clean up query
	query = strings.TrimSpace(query)

	// Remove @ prefix if present
	if strings.HasPrefix(query, "@") {
		query = strings.TrimPrefix(query, "@")
	}

	// Get all users
	usersMap := uh.apiProvider.ProvideUsersMap()

	var matches []UserResolution
	queryLower := strings.ToLower(query)

	// Search through all users
	for userID, user := range usersMap.Users {

		var matchType string
		isMatch := false

		// Check based on search type
		switch searchType {
		case "username":
			if strings.EqualFold(user.Name, query) {
				isMatch = true
				matchType = "username_exact"
			} else if strings.Contains(strings.ToLower(user.Name), queryLower) {
				isMatch = true
				matchType = "username_partial"
			}

		case "display_name":
			// First try DisplayName
			if user.Profile.DisplayName != "" {
				normalizedDisplayName := normalizeString(user.Profile.DisplayName)
				if strings.EqualFold(normalizedDisplayName, query) {
					isMatch = true
					matchType = "display_name_exact"
				} else if strings.Contains(strings.ToLower(normalizedDisplayName), queryLower) {
					isMatch = true
					matchType = "display_name_partial"
				}
			}

			// Fallback to RealName if DisplayName is empty or no match found
			// (Slack UI often shows RealName as display name when DisplayName is not set)
			if !isMatch && user.RealName != "" {
				normalizedRealName := normalizeString(user.RealName)
				if strings.EqualFold(normalizedRealName, query) {
					isMatch = true
					matchType = "real_name_exact"
				} else if strings.Contains(strings.ToLower(normalizedRealName), queryLower) {
					isMatch = true
					matchType = "real_name_partial"
				}
			}

		case "real_name":
			if user.RealName != "" {
				normalizedRealName := normalizeString(user.RealName)
				if strings.EqualFold(normalizedRealName, query) {
					isMatch = true
					matchType = "real_name_exact"
				} else if strings.Contains(strings.ToLower(normalizedRealName), queryLower) {
					isMatch = true
					matchType = "real_name_partial"
				}
			}

		case "email":
			if user.Profile.Email != "" {
				if strings.EqualFold(user.Profile.Email, query) {
					isMatch = true
					matchType = "email_exact"
				} else if strings.Contains(strings.ToLower(user.Profile.Email), queryLower) {
					isMatch = true
					matchType = "email_partial"
				}
			}

		case "auto":
			// Try all methods, prioritizing exact matches
			// Check username exact match
			if strings.EqualFold(user.Name, query) {
				isMatch = true
				matchType = "username_exact"
			} else if user.Profile.DisplayName != "" && strings.EqualFold(normalizeString(user.Profile.DisplayName), query) {
				isMatch = true
				matchType = "display_name_exact"
			} else if user.RealName != "" && strings.EqualFold(normalizeString(user.RealName), query) {
				isMatch = true
				matchType = "real_name_exact"
			} else if user.Profile.Email != "" && strings.EqualFold(user.Profile.Email, query) {
				isMatch = true
				matchType = "email_exact"
			} else {
				// Try partial matches
				if strings.Contains(strings.ToLower(user.Name), queryLower) {
					isMatch = true
					matchType = "username_partial"
				} else if user.Profile.DisplayName != "" && strings.Contains(strings.ToLower(normalizeString(user.Profile.DisplayName)), queryLower) {
					isMatch = true
					matchType = "display_name_partial"
				} else if user.RealName != "" && strings.Contains(strings.ToLower(normalizeString(user.RealName)), queryLower) {
					isMatch = true
					matchType = "real_name_partial"
				} else if user.Profile.Email != "" && strings.Contains(strings.ToLower(user.Profile.Email), queryLower) {
					isMatch = true
					matchType = "email_partial"
				}
			}

		default:
			return nil, fmt.Errorf("invalid search_type: %s. Must be one of: username, display_name, real_name, email, auto", searchType)
		}

		if isMatch {
			resolution := UserResolution{
				UserID:      userID,
				UserName:    user.Name,
				RealName:    user.RealName,
				DisplayName: user.Profile.DisplayName,
				Email:       user.Profile.Email,
				MatchType:   matchType,
				IsBot:       user.IsBot,
			}
			matches = append(matches, resolution)
		}
	}

	// Sort matches by priority (exact matches first)
	sortedMatches := sortUserMatches(matches)

	// Convert to CSV
	csvContent, err := gocsv.MarshalString(&sortedMatches)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal results to CSV: %w", err)
	}

	return mcp.NewToolResultText(csvContent), nil
}

// sortUserMatches sorts user matches by priority: exact matches first, then partial matches
func sortUserMatches(matches []UserResolution) []UserResolution {
	// Simple priority-based sorting
	var exactMatches []UserResolution
	var partialMatches []UserResolution

	for _, match := range matches {
		if strings.Contains(match.MatchType, "_exact") {
			exactMatches = append(exactMatches, match)
		} else {
			partialMatches = append(partialMatches, match)
		}
	}

	// Combine with exact matches first
	result := append(exactMatches, partialMatches...)
	return result
}
