package handler

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"log"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gocarina/gocsv"
	"github.com/korotovsky/slack-mcp-server/pkg/provider"
	"github.com/korotovsky/slack-mcp-server/pkg/text"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/slack-go/slack"
	slackGoUtil "github.com/takara2314/slack-go-util"
)

type Message struct {
	UserID   string `json:"userID"`
	UserName string `json:"userUser"`
	RealName string `json:"realName"`
	Channel  string `json:"channelID"`
	ThreadTs string `json:"ThreadTs"`
	Text     string `json:"text"`
	Time     string `json:"time"`
	Cursor   string `json:"cursor"`
}

type conversationParams struct {
	channel  string
	limit    int
	oldest   string
	latest   string
	cursor   string
	activity bool
}

var validFilterKeys = map[string]struct{}{
	"is":     {},
	"in":     {},
	"from":   {},
	"with":   {},
	"before": {},
	"after":  {},
	"on":     {},
	"during": {},
}

type searchParams struct {
	query string // query:search query
	limit int    // limit:100
	page  int    // page:1
}

type addMessageParams struct {
	channel     string
	threadTs    string
	text        string
	contentType string
}

type createChannelParams struct {
	name string
}

type renameChannelParams struct {
	channel string
	name    string
}

type inviteUsersParams struct {
	channel string
	users   []string
}

type setTopicParams struct {
	channel string
	topic   string
}

type ConversationsHandler struct {
	apiProvider *provider.ApiProvider
}

func NewConversationsHandler(apiProvider *provider.ApiProvider) *ConversationsHandler {
	return &ConversationsHandler{
		apiProvider: apiProvider,
	}
}

func (ch *ConversationsHandler) ConversationsAddMessageHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	params, err := ch.parseParamsToolAddMessage(request)
	if err != nil {
		return nil, err
	}

	api, err := ch.apiProvider.ProvideGeneric()
	if err != nil {
		return nil, err
	}

	var options []slack.MsgOption

	if params.threadTs != "" {
		options = append(options, slack.MsgOptionTS(params.threadTs))
	}

	if params.contentType == "text/plain" {
		options = append(options, slack.MsgOptionDisableMarkdown())
		options = append(options, slack.MsgOptionText(params.text, false))
	} else if params.contentType == "text/markdown" {
		blocks, err := slackGoUtil.ConvertMarkdownTextToBlocks(params.text)
		if err == nil {
			options = append(options, slack.MsgOptionBlocks(blocks...))
		} else {
			// fallback to plain text if conversion fails
			log.Printf("Markdown parsing error: %s\n", err.Error())

			options = append(options, slack.MsgOptionDisableMarkdown())
			options = append(options, slack.MsgOptionText(params.text, false))
		}
	} else {
		return nil, errors.New("content_type must be either 'text/plain' or 'text/markdown'")
	}

	respChannel, respTimestamp, err := api.PostMessageContext(ctx, params.channel, options...)

	if err != nil {
		return nil, err
	}

	historyParams := slack.GetConversationHistoryParameters{
		ChannelID: respChannel,
		Limit:     1,
		Oldest:    respTimestamp,
		Latest:    respTimestamp,
		Inclusive: true,
	}

	history, err := api.GetConversationHistoryContext(ctx, &historyParams)
	if err != nil {
		return nil, err
	}

	messages := ch.convertMessagesFromHistory(history.Messages, historyParams.ChannelID, false)

	return marshalMessagesToCSV(messages)
}

func (ch *ConversationsHandler) ConversationsHistoryHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	params, err := ch.parseParamsToolConversations(request)
	if err != nil {
		return nil, err
	}

	api, err := ch.apiProvider.ProvideGeneric()
	if err != nil {
		return nil, err
	}

	historyParams := slack.GetConversationHistoryParameters{
		ChannelID: params.channel,
		Limit:     params.limit,
		Oldest:    params.oldest,
		Latest:    params.latest,
		Cursor:    params.cursor,
		Inclusive: false,
	}

	history, err := api.GetConversationHistoryContext(ctx, &historyParams)
	if err != nil {
		return nil, err
	}

	messages := ch.convertMessagesFromHistory(history.Messages, params.channel, params.activity)

	if len(messages) > 0 && history.HasMore {
		messages[len(messages)-1].Cursor = history.ResponseMetaData.NextCursor
	}

	return marshalMessagesToCSV(messages)
}

func (ch *ConversationsHandler) ConversationsRepliesHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	params, err := ch.parseParamsToolConversations(request)
	if err != nil {
		return nil, err
	}

	threadTs := request.GetString("thread_ts", "")
	if threadTs == "" {
		return nil, errors.New("thread_ts must be a string")
	}

	api, err := ch.apiProvider.ProvideGeneric()
	if err != nil {
		return nil, err
	}

	repliesParams := slack.GetConversationRepliesParameters{
		ChannelID: params.channel,
		Timestamp: threadTs,
		Limit:     params.limit,
		Oldest:    params.oldest,
		Latest:    params.latest,
		Cursor:    params.cursor,
		Inclusive: false,
	}

	replies, hasMore, nextCursor, err := api.GetConversationRepliesContext(ctx, &repliesParams)
	if err != nil {
		return nil, err
	}

	messages := ch.convertMessagesFromHistory(replies, params.channel, params.activity)

	if len(messages) > 0 && hasMore {
		messages[len(messages)-1].Cursor = nextCursor
	}

	return marshalMessagesToCSV(messages)
}

func (ch *ConversationsHandler) ConversationsSearchHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	params, err := ch.parseParamsToolSearch(request)
	if err != nil {
		return nil, err
	}

	api, err := ch.apiProvider.ProvideGeneric()
	if err != nil {
		return nil, err
	}

	searchParams := slack.SearchParameters{
		Sort:          slack.DEFAULT_SEARCH_SORT,
		SortDirection: slack.DEFAULT_SEARCH_SORT_DIR,
		Highlight:     false,
		Count:         params.limit,
		Page:          params.page,
	}

	messagesRes, _, err := api.SearchContext(ctx, params.query, searchParams)
	if err != nil {
		return nil, err
	}

	messages := ch.convertMessagesFromSearch(messagesRes.Matches)

	if len(messages) > 0 && ((messagesRes.Pagination.PerPage * messagesRes.Pagination.PageCount) < messagesRes.Pagination.TotalCount) {
		nextCursor := fmt.Sprintf("page:%d", messagesRes.Pagination.PageCount+1)

		messages[len(messages)-1].Cursor = base64.StdEncoding.EncodeToString([]byte(nextCursor))
	}

	return marshalMessagesToCSV(messages)
}

func (ch *ConversationsHandler) ConversationsCreateHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	params, err := ch.parseParamsToolCreateChannel(request)
	if err != nil {
		return nil, err
	}

	api, err := ch.apiProvider.ProvideGeneric()
	if err != nil {
		return nil, err
	}

	createParams := slack.CreateConversationParams{
		ChannelName: params.name,
		IsPrivate:   false, // Always create public channels
	}

	channel, err := api.CreateConversationContext(ctx, createParams)
	if err != nil {
		return nil, err
	}

	return mcp.NewToolResultText(fmt.Sprintf("Channel created successfully: %s (ID: %s)", channel.Name, channel.ID)), nil
}

func (ch *ConversationsHandler) ConversationsRenameHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	params, err := ch.parseParamsToolRenameChannel(request)
	if err != nil {
		return nil, err
	}

	api, err := ch.apiProvider.ProvideGeneric()
	if err != nil {
		return nil, err
	}

	channel, err := api.RenameConversationContext(ctx, params.channel, params.name)
	if err != nil {
		return nil, err
	}

	return mcp.NewToolResultText(fmt.Sprintf("Channel renamed successfully to: %s", channel.Name)), nil
}

func (ch *ConversationsHandler) ConversationsInviteHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	params, err := ch.parseParamsToolInviteUsers(request)
	if err != nil {
		return nil, err
	}

	api, err := ch.apiProvider.ProvideGeneric()
	if err != nil {
		return nil, err
	}

	channel, err := api.InviteUsersToConversationContext(ctx, params.channel, params.users...)
	if err != nil {
		return nil, err
	}

	return mcp.NewToolResultText(fmt.Sprintf("Successfully invited %d user(s) to channel: %s", len(params.users), channel.Name)), nil
}

func (ch *ConversationsHandler) ConversationsSetTopicHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	params, err := ch.parseParamsToolSetTopic(request)
	if err != nil {
		return nil, err
	}

	api, err := ch.apiProvider.ProvideGeneric()
	if err != nil {
		return nil, err
	}

	channel, err := api.SetTopicOfConversationContext(ctx, params.channel, params.topic)
	if err != nil {
		return nil, err
	}

	return mcp.NewToolResultText(fmt.Sprintf("Successfully set topic for channel %s: %s", channel.Name, params.topic)), nil
}

func isChannelAllowed(channel string) bool {
	config := os.Getenv("SLACK_MCP_ADD_MESSAGE_TOOL")
	if config == "" || config == "true" || config == "1" {
		return true
	}

	items := strings.Split(config, ",")
	isNegated := strings.HasPrefix(strings.TrimSpace(items[0]), "!")

	for _, item := range items {
		item = strings.TrimSpace(item)
		if isNegated {
			if strings.TrimPrefix(item, "!") == channel {
				return false
			}
		} else {
			if item == channel {
				return true
			}
		}
	}

	return !isNegated
}

func (ch *ConversationsHandler) convertMessagesFromHistory(slackMessages []slack.Message, channel string, includeActivity bool) []Message {
	usersMap := ch.apiProvider.ProvideUsersMap()
	var messages []Message

	for _, msg := range slackMessages {
		if msg.SubType != "" && !includeActivity {
			continue
		}

		userName, realName := getUserInfo(msg.User, usersMap.Users)

		messages = append(messages, Message{
			UserID:   msg.User,
			UserName: userName,
			RealName: realName,
			Text:     text.ProcessText(msg.Text),
			Channel:  channel,
			ThreadTs: msg.ThreadTimestamp,
			Time:     msg.Timestamp,
		})
	}

	return messages
}

func (ch *ConversationsHandler) convertMessagesFromSearch(slackMessages []slack.SearchMessage) []Message {
	usersMap := ch.apiProvider.ProvideUsersMap()
	var messages []Message

	for _, msg := range slackMessages {
		userName, realName := getUserInfo(msg.User, usersMap.Users)
		threadTs, _ := extractThreadTS(msg.Permalink)

		messages = append(messages, Message{
			UserID:   msg.User,
			UserName: userName,
			RealName: realName,
			Text:     text.ProcessText(msg.Text),
			Channel:  fmt.Sprintf("#%s", msg.Channel.Name),
			ThreadTs: threadTs,
			Time:     msg.Timestamp,
		})
	}

	return messages
}

func (ch *ConversationsHandler) parseParamsToolConversations(request mcp.CallToolRequest) (*conversationParams, error) {
	channel := request.GetString("channel_id", "")
	if channel == "" {
		return nil, errors.New("channel_id must be a string")
	}

	limit := request.GetString("limit", "")
	cursor := request.GetString("cursor", "")
	activity := request.GetBool("include_activity_messages", false)

	var (
		paramLimit  int
		paramOldest string
		paramLatest string
		err         error
	)

	if strings.HasSuffix(limit, "d") {
		paramLimit, paramOldest, paramLatest, err = limitByDays(limit)
		if err != nil {
			return nil, err
		}
	} else if cursor == "" {
		paramLimit, err = limitByNumeric(limit)
		if err != nil {
			return nil, err
		}
	}

	if strings.HasPrefix(channel, "#") || strings.HasPrefix(channel, "@") {
		channelsMaps := ch.apiProvider.ProvideChannelsMaps()
		chn, ok := channelsMaps.ChannelsInv[channel]
		if !ok {
			return nil, fmt.Errorf("channel %q not found", channel)
		}

		channel = channelsMaps.Channels[chn].ID
	}

	return &conversationParams{
		channel:  channel,
		limit:    paramLimit,
		oldest:   paramOldest,
		latest:   paramLatest,
		cursor:   cursor,
		activity: activity,
	}, nil
}

func (ch *ConversationsHandler) parseParamsToolAddMessage(request mcp.CallToolRequest) (*addMessageParams, error) {
	toolConfig := os.Getenv("SLACK_MCP_ADD_MESSAGE_TOOL")
	if toolConfig == "" {
		return nil, errors.New("by default, the conversations_add_message tool is disabled to guard Slack workspaces against accidental spamming. To enable it, set the SLACK_MCP_ADD_MESSAGE_TOOL environment variable to true, 1, or comma separated list of channels to limit where the MCP can post messages, e.g. 'SLACK_MCP_ADD_MESSAGE_TOOL=C1234567890,D0987654321', 'SLACK_MCP_ADD_MESSAGE_TOOL=!C1234567890' to enable all except one or 'SLACK_MCP_ADD_MESSAGE_TOOL=true' for all channels and DMs")
	}

	channel := request.GetString("channel_id", "")
	if channel == "" {
		return nil, errors.New("channel_id must be a string")
	}

	if strings.HasPrefix(channel, "#") || strings.HasPrefix(channel, "@") {
		channelsMaps := ch.apiProvider.ProvideChannelsMaps()
		chn, ok := channelsMaps.ChannelsInv[channel]
		if !ok {
			return nil, fmt.Errorf("channel %q not found", channel)
		}

		channel = channelsMaps.Channels[chn].ID
	}

	if !isChannelAllowed(channel) {
		return nil, fmt.Errorf("conversations_add_message tool is not allowed for channel %q, applied policy: %s", channel, toolConfig)
	}

	threadTs := request.GetString("thread_ts", "")
	if threadTs != "" && !strings.Contains(threadTs, ".") {
		return nil, errors.New("thread_ts must be a valid timestamp in format 1234567890.123456")
	}

	msgText := request.GetString("payload", "")
	if msgText == "" {
		return nil, errors.New("text must be a string")
	}

	contentType := request.GetString("content_type", "text/markdown")
	if contentType != "text/plain" && contentType != "text/markdown" {
		return nil, errors.New("content_type must be either 'text/plain' or 'text/markdown'")
	}

	return &addMessageParams{
		channel:     channel,
		threadTs:    threadTs,
		text:        msgText,
		contentType: contentType,
	}, nil
}

func (ch *ConversationsHandler) parseParamsToolSearch(req mcp.CallToolRequest) (*searchParams, error) {
	rawQuery := strings.TrimSpace(req.GetString("search_query", ""))

	freeText, filters := splitQuery(rawQuery)

	// is:thread
	if req.GetBool("filter_threads_only", false) {
		addFilter(filters, "is", "thread")
	}

	// in:channel or in:IM
	if chName := req.GetString("filter_in_channel", ""); chName != "" {
		f, err := ch.paramFormatChannel(chName)
		if err != nil {
			return nil, err
		}
		addFilter(filters, "in", f)
	} else if im := req.GetString("filter_in_im_or_mpim", ""); im != "" {
		f, err := ch.paramFormatUser(im)
		if err != nil {
			return nil, err
		}
		addFilter(filters, "in", f)
	}

	// with:
	if with := req.GetString("filter_users_with", ""); with != "" {
		f, err := ch.paramFormatUser(with)
		if err != nil {
			return nil, err
		}
		addFilter(filters, "with", f)
	}

	// from:
	if from := req.GetString("filter_users_from", ""); from != "" {
		f, err := ch.paramFormatUser(from)
		if err != nil {
			return nil, err
		}
		addFilter(filters, "from", f)
	}

	// date filters
	dateMap, err := buildDateFilters(
		req.GetString("filter_date_before", ""),
		req.GetString("filter_date_after", ""),
		req.GetString("filter_date_on", ""),
		req.GetString("filter_date_during", ""),
	)
	if err != nil {
		return nil, err
	}
	for key, val := range dateMap {
		addFilter(filters, key, val)
	}

	finalQuery := buildQuery(freeText, filters)

	limit := req.GetInt("limit", 100)
	cursor := req.GetString("cursor", "")

	var (
		page          int
		decodedCursor []byte
	)
	if cursor != "" {
		decodedCursor, err = base64.StdEncoding.DecodeString(cursor)
		if err != nil {
			return nil, fmt.Errorf("invalid cursor: %v", err)
		}
		partOfCursor := strings.Split(string(decodedCursor), ":")
		if len(partOfCursor) != 2 {
			return nil, fmt.Errorf("invalid cursor: %v", cursor)
		}
		page, err = strconv.Atoi(partOfCursor[1])
		if err != nil || page < 1 {
			return nil, fmt.Errorf("invalid cursor page: %v", err)
		}
	} else {
		page = 1
	}

	return &searchParams{
		query: finalQuery,
		limit: limit,
		page:  page,
	}, nil
}

func (ch *ConversationsHandler) paramFormatUser(raw string) (string, error) {
	users := ch.apiProvider.ProvideUsersMap()

	raw = strings.TrimSpace(raw)
	if strings.HasPrefix(raw, "U") {
		u, ok := users.Users[raw]
		if !ok {
			return "", fmt.Errorf("user %q not found", raw)
		}

		return fmt.Sprintf("<@%s>", u.ID), nil
	} else {
		if strings.HasPrefix(raw, "<@") {
			return raw, nil
		}
		if strings.HasPrefix(raw, "@") {
			raw = raw[1:]
		}
		u, ok := users.UsersInv[raw]
		if !ok {
			return "", fmt.Errorf("user %q not found", raw)
		}
		return fmt.Sprintf("@%s", users.Users[u].Name), nil
	}
}

func (ch *ConversationsHandler) paramFormatChannel(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	cms := ch.apiProvider.ProvideChannelsMaps()
	if strings.HasPrefix(raw, "#") {
		if chID, ok := cms.ChannelsInv[raw]; ok {
			return "#" + cms.Channels[chID].Name, nil
		}
		return "", fmt.Errorf("channel %q not found", raw)
	}
	if strings.HasPrefix(raw, "C") {
		if chn, ok := cms.Channels[raw]; ok {
			return "#" + chn.Name, nil
		}
		return "", fmt.Errorf("channel %q not found", raw)
	}
	return "", fmt.Errorf("invalid channel format: %q", raw)
}

func marshalMessagesToCSV(messages []Message) (*mcp.CallToolResult, error) {
	csvBytes, err := gocsv.MarshalBytes(&messages)
	if err != nil {
		return nil, err
	}
	return mcp.NewToolResultText(string(csvBytes)), nil
}

func getUserInfo(userID string, usersMap map[string]slack.User) (userName, realName string) {
	if user, ok := usersMap[userID]; ok {
		return user.Name, user.RealName
	}
	return userID, userID
}

func limitByNumeric(limit string) (int, error) {
	n, err := strconv.Atoi(limit)
	if err != nil {
		return 0, fmt.Errorf("invalid numeric limit: %q", limit)
	}
	return n, nil
}

// limitByDays parses a string like "1d", "2d", etc.
// It returns:
//   - the per page limit (100)
//   - oldest timestamp = midnight of (today − days + 1),
//   - latest timestamp = now,
//   - or an error if parsing fails.
func limitByDays(limit string) (slackLimit int, oldest, latest string, err error) {
	daysStr := strings.TrimSuffix(limit, "d")
	days, err := strconv.Atoi(daysStr)
	if err != nil || days <= 0 {
		return 0, "", "", fmt.Errorf("invalid duration limit %q: must be a positive integer with 'd' suffix", limit)
	}

	now := time.Now()
	loc := now.Location()

	startOfToday := time.Date(
		now.Year(), now.Month(), now.Day(),
		0, 0, 0, 0,
		loc,
	)

	oldestTime := startOfToday.AddDate(0, 0, -days+1)

	latest = fmt.Sprintf("%d.000000", now.Unix())
	oldest = fmt.Sprintf("%d.000000", oldestTime.Unix())

	return 100, oldest, latest, nil
}

func extractThreadTS(rawurl string) (string, error) {
	u, err := url.Parse(rawurl)
	if err != nil {
		return "", err
	}
	return u.Query().Get("thread_ts"), nil
}

func parseFlexibleDate(in string) (time.Time, time.Time, error) {
	today := time.Now().Truncate(24 * time.Hour)
	s := strings.ToLower(strings.TrimSpace(in))
	switch s {
	case "today":
		return today, today, nil
	case "yesterday":
		y := today.AddDate(0, 0, -1)
		return y, y, nil
	}
	// exact date
	if d, err := time.Parse("2006-01-02", s); err == nil {
		return d, d, nil
	}
	// month name
	if m, err := time.Parse("January", strings.Title(s)); err == nil {
		year := time.Now().Year()
		start := time.Date(year, m.Month(), 1, 0, 0, 0, 0, time.Local)
		end := start.AddDate(0, 1, -1)
		return start, end, nil
	}
	return time.Time{}, time.Time{}, fmt.Errorf("invalid date: %q", in)
}

func buildDateFilters(before, after, on, during string) (map[string]string, error) {
	out := make(map[string]string)

	if on != "" {
		if during != "" || before != "" || after != "" {
			return nil, fmt.Errorf("'on' cannot be combined with other date filters")
		}
		if _, _, err := parseFlexibleDate(on); err != nil {
			return nil, fmt.Errorf("invalid 'on' date: %v", err)
		}
		out["on"] = on
		return out, nil
	}
	if during != "" {
		if before != "" || after != "" {
			return nil, fmt.Errorf("'during' cannot be combined with 'before' or 'after'")
		}
		if _, _, err := parseFlexibleDate(during); err != nil {
			return nil, fmt.Errorf("invalid 'during' date: %v", err)
		}
		out["during"] = during
		return out, nil
	}
	if after != "" {
		if _, _, err := parseFlexibleDate(after); err != nil {
			return nil, fmt.Errorf("invalid 'after' date: %v", err)
		}
		out["after"] = after
	}
	if before != "" {
		if _, _, err := parseFlexibleDate(before); err != nil {
			return nil, fmt.Errorf("invalid 'before' date: %v", err)
		}
		out["before"] = before
	}
	if after != "" && before != "" {
		a, _, _ := parseFlexibleDate(after)
		b, _, _ := parseFlexibleDate(before)
		if a.After(b) {
			return nil, fmt.Errorf("'after' date is after 'before' date")
		}
	}
	return out, nil
}

func isFilterKey(key string) bool {
	_, ok := validFilterKeys[strings.ToLower(key)]
	return ok
}

func splitQuery(q string) (freeText []string, filters map[string][]string) {
	filters = make(map[string][]string)
	for _, tok := range strings.Fields(q) {
		parts := strings.SplitN(tok, ":", 2)
		if len(parts) == 2 && isFilterKey(parts[0]) {
			key := strings.ToLower(parts[0])
			filters[key] = append(filters[key], parts[1])
		} else {
			freeText = append(freeText, tok)
		}
	}
	return
}

func addFilter(filters map[string][]string, key, val string) {
	for _, existing := range filters[key] {
		if existing == val {
			return
		}
	}
	filters[key] = append(filters[key], val)
}

func buildQuery(freeText []string, filters map[string][]string) string {
	out := make([]string, 0, len(freeText)+len(filters)*2)
	out = append(out, freeText...)
	for _, key := range []string{"is", "in", "from", "with", "before", "after", "on", "during"} {
		for _, val := range filters[key] {
			out = append(out, fmt.Sprintf("%s:%s", key, val))
		}
	}
	return strings.Join(out, " ")
}

func (ch *ConversationsHandler) parseParamsToolCreateChannel(request mcp.CallToolRequest) (*createChannelParams, error) {
	name := request.GetString("name", "")
	if name == "" {
		return nil, errors.New("name must be a string")
	}

	// Validate channel name (Slack channel naming rules)
	if len(name) > 80 {
		return nil, errors.New("channel name must be 80 characters or less")
	}

	return &createChannelParams{
		name: name,
	}, nil
}

func (ch *ConversationsHandler) parseParamsToolRenameChannel(request mcp.CallToolRequest) (*renameChannelParams, error) {
	channel := request.GetString("channel_id", "")
	if channel == "" {
		return nil, errors.New("channel_id must be a string")
	}

	name := request.GetString("name", "")
	if name == "" {
		return nil, errors.New("name must be a string")
	}

	// Validate channel name (Slack channel naming rules)
	if len(name) > 80 {
		return nil, errors.New("channel name must be 80 characters or less")
	}

	// Convert channel name to ID if necessary
	if strings.HasPrefix(channel, "#") {
		channelsMaps := ch.apiProvider.ProvideChannelsMaps()
		chn, ok := channelsMaps.ChannelsInv[channel]
		if !ok {
			return nil, fmt.Errorf("channel %q not found", channel)
		}
		channel = channelsMaps.Channels[chn].ID
	}

	return &renameChannelParams{
		channel: channel,
		name:    name,
	}, nil
}

func (ch *ConversationsHandler) parseParamsToolInviteUsers(request mcp.CallToolRequest) (*inviteUsersParams, error) {
	channel := request.GetString("channel_id", "")
	if channel == "" {
		return nil, errors.New("channel_id must be a string")
	}

	usersStr := request.GetString("users", "")
	if usersStr == "" {
		return nil, errors.New("users must be a comma-separated string of user IDs")
	}

	// Convert channel name to ID if necessary
	if strings.HasPrefix(channel, "#") {
		channelsMaps := ch.apiProvider.ProvideChannelsMaps()
		chn, ok := channelsMaps.ChannelsInv[channel]
		if !ok {
			return nil, fmt.Errorf("channel %q not found", channel)
		}
		channel = channelsMaps.Channels[chn].ID
	}

	// Parse user IDs
	users := strings.Split(usersStr, ",")
	for i, user := range users {
		users[i] = strings.TrimSpace(user)
		
		// Convert @username to user ID if necessary
		if strings.HasPrefix(users[i], "@") {
			usersMap := ch.apiProvider.ProvideUsersMap()
			userName := strings.TrimPrefix(users[i], "@")
			userID, ok := usersMap.UsersInv[userName]
			if !ok {
				return nil, fmt.Errorf("user %q not found", users[i])
			}
			users[i] = userID
		}
	}

	return &inviteUsersParams{
		channel: channel,
		users:   users,
	}, nil
}

func (ch *ConversationsHandler) parseParamsToolSetTopic(request mcp.CallToolRequest) (*setTopicParams, error) {
	channel := request.GetString("channel_id", "")
	if channel == "" {
		return nil, errors.New("channel_id must be a string")
	}

	topic := request.GetString("topic", "")
	if topic == "" {
		return nil, errors.New("topic must be a string")
	}

	// Convert channel name to ID if necessary
	if strings.HasPrefix(channel, "#") {
		channelsMaps := ch.apiProvider.ProvideChannelsMaps()
		chn, ok := channelsMaps.ChannelsInv[channel]
		if !ok {
			return nil, fmt.Errorf("channel %q not found", channel)
		}
		channel = channelsMaps.Channels[chn].ID
	}

	return &setTopicParams{
		channel: channel,
		topic:   topic,
	}, nil
}
