package executor

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/just4zeroq/Omni-link/translator"
)

func init() {
	Register("openai", &OpenAIExecutor{})
}

// OpenAIExecutor handles pure OpenAI-compatible endpoints.
// Native format: openai (passthrough).
type OpenAIExecutor struct {
	channel any
}

func (e *OpenAIExecutor) Init(channel any) {
	e.channel = channel
}

func (e *OpenAIExecutor) GetName() string {
	if ch, ok := e.channel.(interface{ GetName() string }); ok {
		return ch.GetName()
	}
	return "OpenAI"
}

func (e *OpenAIExecutor) NativeFormats() []EndpointCapability {
	return []EndpointCapability{
		{Format: translator.FormatOpenAI, RelayMode: translator.RelayModeChatCompletions},
	}
}

func (e *OpenAIExecutor) GetRequestURL(info *RequestInfo) (string, error) {
	baseURL := info.BaseURL
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}
	baseURL = strings.TrimSuffix(baseURL, "/")
	return baseURL + "/chat/completions", nil
}

func (e *OpenAIExecutor) SetupRequestHeader(header http.Header, info *RequestInfo) error {
	header.Set("Authorization", "Bearer "+info.ApiKey)
	header.Set("Content-Type", "application/json")
	if info.IsStream {
		header.Set("Accept", "text/event-stream")
	} else {
		header.Set("Accept", "application/json")
	}
	return nil
}

func (e *OpenAIExecutor) ConvertRequest(body []byte, from, to translator.Format) ([]byte, error) {
	if from == to {
		return body, nil
	}

	switch {
	case from == translator.FormatClaude && to == translator.FormatOpenAI:
		return oaClaudeToOpenAIRequest(body)
	case from == translator.FormatOpenAIResponses && to == translator.FormatOpenAI:
		return ResponsesToOpenAIRequest(body)
	default:
		return nil, fmt.Errorf("openai: unsupported request conversion %s→%s", from, to)
	}
}

func (e *OpenAIExecutor) ConvertResponse(body []byte, from, to translator.Format) ([]byte, error) {
	if from == to {
		return body, nil
	}

	switch {
	case from == translator.FormatOpenAI && to == translator.FormatClaude:
		return oaOpenAIToClaudeResponse(body)
	case from == translator.FormatOpenAI && to == translator.FormatOpenAIResponses:
		return OpenAIToResponsesResponse(body)
	default:
		return nil, fmt.Errorf("openai: unsupported response conversion %s→%s", from, to)
	}
}

func (e *OpenAIExecutor) RequestCustomize(body []byte, info *RequestInfo) []byte {
	if info.ActualModelName != "" {
		body = replaceModelField(body, info.ActualModelName)
	}
	if info.IsStream {
		body = injectStreamOptionsOpenAI(body)
	}
	return body
}

func (e *OpenAIExecutor) ResponseCustomize(body []byte, info *RequestInfo) []byte {
	return body
}

func (e *OpenAIExecutor) NewResponseStream(from, to translator.Format) (ResponseStream, error) {
	if from == to {
		return nil, nil
	}

	switch {
	case from == translator.FormatOpenAI && to == translator.FormatClaude:
		return newOpenAIToClaudeStream(), nil
	case from == translator.FormatOpenAI && to == translator.FormatOpenAIResponses:
		// OpenAI → Responses streaming not yet implemented
		return nil, nil
	default:
		return nil, fmt.Errorf("openai: streaming conversion %s→%s not implemented", from, to)
	}
}

func (e *OpenAIExecutor) DoRequest(info *RequestInfo, body io.Reader) (*http.Response, error) {
	reqURL, err := e.GetRequestURL(info)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequest("POST", reqURL, body)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	if err := e.SetupRequestHeader(httpReq.Header, info); err != nil {
		return nil, err
	}

	client := &http.Client{}
	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}
	return resp, nil
}

// ========================================================================
// Responses API ↔ OpenAI Chat (openai-only, no conflicts)
// ========================================================================

// ResponsesToOpenAIRequest converts Responses API request to OpenAI Chat format.
func ResponsesToOpenAIRequest(body []byte) ([]byte, error) {
	var req translator.ResponsesRequest
	if err := json.Unmarshal(body, &req); err != nil {
		return nil, fmt.Errorf("responses→openai: unmarshal: %w", err)
	}

	chatReq := translator.ChatRequest{
		Model: req.Model,
	}
	if req.Temperature != nil {
		chatReq.Temperature = req.Temperature
	}
	if req.TopP != nil {
		chatReq.TopP = req.TopP
	}
	if req.MaxOutputTokens != nil {
		chatReq.MaxCompletionTokens = oaIntPtr(int(*req.MaxOutputTokens))
	}
	if req.Stream != nil {
		chatReq.Stream = req.Stream
	}

	chatReq.Messages = make([]translator.Message, 0)

	if len(req.Instructions) > 0 {
		var instr string
		if err := json.Unmarshal(req.Instructions, &instr); err == nil && instr != "" {
			chatReq.Messages = append(chatReq.Messages, translator.Message{
				Role: "system", Content: instr,
			})
		}
	}

	if len(req.Input) > 0 {
		messages, err := parseResponsesInput(req.Input)
		if err == nil {
			chatReq.Messages = append(chatReq.Messages, messages...)
		}
	}

	chatReq.Tools = req.Tools
	chatReq.ToolChoice = mapResponseToolChoice(req.ToolChoice)

	return json.Marshal(chatReq)
}

// OpenAIToResponsesResponse converts OpenAI Chat response to Responses API format.
func OpenAIToResponsesResponse(body []byte) ([]byte, error) {
	var chatResp translator.ChatResponse
	if err := json.Unmarshal(body, &chatResp); err != nil {
		return nil, fmt.Errorf("openai→responses: unmarshal: %w", err)
	}

	resp := translator.ResponsesResponse{
		ID:     chatResp.ID,
		Object: "response",
		Status: "completed",
		Model:  chatResp.Model,
	}

	for _, choice := range chatResp.Choices {
		content := make([]translator.ResponsesOutputContent, 0)
		if choice.Message.Content != nil && *choice.Message.Content != "" {
			content = append(content, translator.ResponsesOutputContent{
				Type: "output_text",
				Text: *choice.Message.Content,
			})
		}
		if len(choice.Message.ToolCalls) > 0 {
			for _, tc := range choice.Message.ToolCalls {
				resp.Output = append(resp.Output, translator.ResponsesOutput{
					Type:      "function_call",
					ID:        tc.ID,
					CallID:    tc.ID,
					Name:      tc.Function.Name,
					Arguments: tc.Function.Arguments,
					Status:    "completed",
				})
			}
		}
		if len(content) > 0 {
			resp.Output = append(resp.Output, translator.ResponsesOutput{
				Type:    "message",
				ID:      "msg_" + chatResp.ID,
				Status:  "completed",
				Role:    "assistant",
				Content: content,
			})
		}
	}

	if chatResp.Usage != nil {
		resp.Usage = &translator.ResponsesUsage{
			InputTokens:  chatResp.Usage.PromptTokens,
			OutputTokens: chatResp.Usage.CompletionTokens,
			TotalTokens:  chatResp.Usage.TotalTokens,
		}
	}

	return json.Marshal(resp)
}

func parseResponsesInput(input json.RawMessage) ([]translator.Message, error) {
	var str string
	if err := json.Unmarshal(input, &str); err == nil {
		return []translator.Message{{Role: "user", Content: str}}, nil
	}

	var items []translator.InputItem
	if err := json.Unmarshal(input, &items); err != nil {
		var raw []map[string]any
		if err := json.Unmarshal(input, &raw); err != nil {
			return nil, err
		}
		items = make([]translator.InputItem, 0, len(raw))
		for _, r := range raw {
			item := translator.InputItem{}
			if t, ok := r["type"].(string); ok {
				item.Type = t
			}
			if role, ok := r["role"].(string); ok {
				item.Role = role
				if item.Type == "" {
					item.Type = "message"
				}
			}
			if text, ok := r["content"].(string); ok {
				item.Content = text
			}
			items = append(items, item)
		}
	}

	msgs := make([]translator.Message, 0, len(items))
	for _, item := range items {
		switch item.Type {
		case "input_text", "":
			msgs = append(msgs, translator.Message{Role: "user", Content: item.Text})
		case "message":
			role := item.Role
			if role == "" {
				role = "user"
			}
			if item.Content != "" {
				msgs = append(msgs, translator.Message{Role: role, Content: item.Content})
			}
		case "function_call_output":
			msgs = append(msgs, translator.Message{
				Role:       "tool",
				ToolCallID: item.CallID,
				Content:    item.Output,
			})
		case "item_reference":
			continue
		}
	}
	return msgs, nil
}

func mapResponseToolChoice(tc json.RawMessage) any {
	if len(tc) == 0 {
		return nil
	}
	var choice string
	if err := json.Unmarshal(tc, &choice); err == nil {
		return choice
	}
	var obj map[string]any
	if err := json.Unmarshal(tc, &obj); err == nil {
		return obj
	}
	return nil
}

func replaceModelField(body []byte, model string) []byte {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(body, &raw); err != nil {
		return body
	}
	modelData, _ := json.Marshal(model)
	raw["model"] = modelData
	result, _ := json.Marshal(raw)
	return result
}

func injectStreamOptionsOpenAI(body []byte) []byte {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(body, &raw); err != nil {
		return body
	}
	if streamRaw, ok := raw["stream"]; ok {
		var stream bool
		if json.Unmarshal(streamRaw, &stream) == nil && stream {
			if _, exists := raw["stream_options"]; !exists {
				opts, _ := json.Marshal(map[string]bool{"include_usage": true})
				raw["stream_options"] = opts
			}
		}
	}
	result, _ := json.Marshal(raw)
	return result
}

// ========================================================================
// Claude → OpenAI conversions (openai-specific, oa prefix)
// ========================================================================

func oaClaudeToOpenAIRequest(body []byte) ([]byte, error) {
	var req translator.ClaudeRequest
	if err := json.Unmarshal(body, &req); err != nil {
		return nil, fmt.Errorf("claude→openai: unmarshal request: %w", err)
	}

	openai := translator.ChatRequest{
		Model: req.Model,
	}
	if req.MaxTokens != nil {
		openai.MaxTokens = oaIntPtr(int(*req.MaxTokens))
	}
	if req.Temperature != nil {
		openai.Temperature = req.Temperature
	}
	if req.TopP != nil {
		openai.TopP = req.TopP
	}
	if req.Stream != nil {
		openai.Stream = req.Stream
	}

	openai.Messages = make([]translator.Message, 0)
	if req.System != nil {
		systemText := oaExtractTextContent(req.System)
		if systemText != "" {
			openai.Messages = append(openai.Messages, translator.Message{
				Role: "system", Content: systemText,
			})
		}
	}

	for _, cm := range req.Messages {
		om := translator.Message{Role: oaMapClaudeRoleToOpenAI(cm.Role)}
		switch content := cm.Content.(type) {
		case string:
			om.Content = content
		case []translator.ClaudeContentBlock:
			om.Content = oaConvertClaudeContentToOpenAI(content)
			for _, block := range content {
				if block.Type == "tool_result" {
					om.Role = "tool"
					om.ToolCallID = block.ToolUseID
					if str, ok := block.Content.(string); ok {
						om.Content = str
					} else {
						b, _ := json.Marshal(block.Content)
						om.Content = string(b)
					}
					break
				}
			}
		default:
			om.Content = fmt.Sprintf("%v", cm.Content)
		}
		openai.Messages = append(openai.Messages, om)
	}

	if len(req.Tools) > 0 {
		raw, _ := json.Marshal(req.Tools)
		openai.Tools = raw
	}
	if req.ToolChoice != nil {
		openai.ToolChoice = oaMapToolChoiceClaudeToOpenAI(req.ToolChoice)
	}
	if req.Thinking != nil && req.Thinking.Type == "enabled" {
		openai.ReasoningEffort = "medium"
	}
	return json.Marshal(openai)
}

func oaOpenAIToClaudeResponse(body []byte) ([]byte, error) {
	var resp translator.ChatResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("openai→claude: unmarshal response: %w", err)
	}

	claudeResp := translator.ClaudeResponse{
		ID:    "",
		Type:  "message",
		Role:  "assistant",
		Model: resp.Model,
	}

	for _, choice := range resp.Choices {
		blocks := make([]translator.ClaudeContentBlock, 0)
		if choice.Message.ReasoningContent != nil && *choice.Message.ReasoningContent != "" {
			blocks = append(blocks, translator.ClaudeContentBlock{
				Type: "thinking", Thinking: oaStrPtr(*choice.Message.ReasoningContent),
			})
		}
		if choice.Message.Content != nil {
			blocks = append(blocks, translator.ClaudeContentBlock{
				Type: "text", Text: choice.Message.Content,
			})
		}
		for _, tc := range choice.Message.ToolCalls {
			blocks = append(blocks, oaClaudeToolUseFromOpenAI(tc))
		}
		claudeResp.Content = blocks
		claudeResp.StopReason = oaMapFinishReasonToClaude(choice.FinishReason)
	}

	if resp.Usage != nil {
		claudeResp.Usage = &translator.ClaudeUsage{
			InputTokens:  resp.Usage.PromptTokens,
			OutputTokens: resp.Usage.CompletionTokens,
		}
	}
	if resp.ID != "" {
		claudeResp.ID = "msg_" + resp.ID
	}
	return json.Marshal(claudeResp)
}

// ========================================================================
// Internal helpers (oa-prefixed, openai-specific)
// ========================================================================

func oaExtractTextContent(content any) string {
	if content == nil {
		return ""
	}
	switch v := content.(type) {
	case string:
		return v
	case []translator.ContentPart:
		var parts []string
		for _, p := range v {
			if p.Text != "" {
				parts = append(parts, p.Text)
			}
		}
		return strings.Join(parts, "\n")
	default:
		b, err := json.Marshal(v)
		if err != nil {
			return fmt.Sprintf("%v", v)
		}
		return string(b)
	}
}

func oaConvertClaudeContentToOpenAI(content []translator.ClaudeContentBlock) any {
	parts := make([]translator.ContentPart, 0, len(content))
	for _, block := range content {
		switch block.Type {
		case "text":
			if block.Text != nil {
				parts = append(parts, translator.ContentPart{Type: "text", Text: *block.Text})
			}
		case "image":
			if block.Source != nil {
				parts = append(parts, translator.ContentPart{
					Type: "image_url",
					ImageURL: &translator.ImageURL{URL: block.Source.URL},
				})
			}
		}
	}
	if len(parts) == 1 {
		return parts[0].Text
	}
	if len(parts) == 0 {
		return ""
	}
	return parts
}

func oaClaudeToolUseFromOpenAI(tc translator.ToolCall) translator.ClaudeContentBlock {
	var input any
	if err := json.Unmarshal([]byte(tc.Function.Arguments), &input); err != nil {
		input = tc.Function.Arguments
	}
	return translator.ClaudeContentBlock{
		Type: "tool_use", ID: tc.ID, Name: tc.Function.Name, Input: input,
	}
}

func oaMapToolChoiceClaudeToOpenAI(tc any) any {
	switch v := tc.(type) {
	case string:
		switch v {
		case "any":
			return "required"
		case "auto":
			return "auto"
		case "none":
			return "none"
		default:
			return "auto"
		}
	case map[string]any:
		typeStr, _ := v["type"].(string)
		if typeStr == "tool" {
			name, _ := v["name"].(string)
			return translator.ToolChoice{Type: "tool", Name: name}
		}
		return typeStr
	case translator.ClaudeToolChoice:
		if v.Type == "tool" && v.Name != "" {
			return translator.ToolChoice{Type: "tool", Name: v.Name}
		}
		return v.Type
	default:
		return "auto"
	}
}

func oaMapClaudeRoleToOpenAI(role string) string {
	switch role {
	case "user", "assistant":
		return role
	default:
		return "user"
	}
}

func oaMapFinishReasonToClaude(reason string) *string {
	switch reason {
	case translator.FinishReasonStop:
		return oaStrPtr("end_turn")
	case translator.FinishReasonToolCalls:
		return oaStrPtr("tool_use")
	case translator.FinishReasonLength:
		return oaStrPtr("max_tokens")
	default:
		return oaStrPtr(reason)
	}
}

func oaStrPtr(s string) *string {
	return &s
}

func oaIntPtr(i int) *int {
	return &i
}
