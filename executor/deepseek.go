package executor

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/just4zeroq/Omni-link/translator"
)

func init() {
	Register("deepseek", &DeepSeekExecutor{})
}

// DeepSeekExecutor handles DeepSeek API with OpenAI and Claude endpoints.
// Native formats: openai (chat completions), claude (messages).
type DeepSeekExecutor struct {
	channel any
}

func (e *DeepSeekExecutor) Init(channel any) {
	e.channel = channel
}

func (e *DeepSeekExecutor) GetName() string {
	if ch, ok := e.channel.(interface{ GetName() string }); ok {
		return ch.GetName()
	}
	return "DeepSeek"
}

func (e *DeepSeekExecutor) NativeFormats() []EndpointCapability {
	return []EndpointCapability{
		{Format: translator.FormatOpenAI, RelayMode: translator.RelayModeChatCompletions},
		{Format: translator.FormatClaude, RelayMode: translator.RelayModeClaudeMessages},
	}
}

func (e *DeepSeekExecutor) GetRequestURL(info *RequestInfo) (string, error) {
	baseURL := info.BaseURL
	if baseURL == "" {
		baseURL = "https://api.deepseek.com"
	}
	baseURL = strings.TrimSuffix(baseURL, "/")

	switch info.RelayMode {
	case translator.RelayModeChatCompletions:
		return baseURL + "/v1/chat/completions", nil
	case translator.RelayModeClaudeMessages:
		return baseURL + "/anthropic/v1/messages", nil
	default:
		return baseURL + "/v1/chat/completions", nil
	}
}

func (e *DeepSeekExecutor) SetupRequestHeader(header http.Header, info *RequestInfo) error {
	header.Set("Authorization", "Bearer "+info.ApiKey)
	header.Set("Content-Type", "application/json")
	if info.IsStream {
		header.Set("Accept", "text/event-stream")
	} else {
		header.Set("Accept", "application/json")
	}
	return nil
}

func (e *DeepSeekExecutor) ConvertRequest(body []byte, from, to translator.Format) ([]byte, error) {
	if from == to {
		return body, nil
	}

	switch {
	case from == translator.FormatOpenAI && to == translator.FormatClaude:
		return dsOpenAIToClaudeRequest(body)
	case from == translator.FormatClaude && to == translator.FormatOpenAI:
		return dsClaudeToOpenAIRequest(body)
	case from == translator.FormatOpenAI && to == translator.FormatOpenAIResponses:
		return ResponsesToOpenAIRequest(body)
	case from == translator.FormatOpenAIResponses && to == translator.FormatOpenAI:
		return ResponsesToOpenAIRequest(body)
	case from == translator.FormatClaude && to == translator.FormatOpenAIResponses:
		return dsClaudeToResponsesRequest(body)
	case from == translator.FormatOpenAIResponses && to == translator.FormatClaude:
		return ResponsesToClaudeRequest(body)
	default:
		return nil, fmt.Errorf("deepseek: unsupported request conversion %s→%s", from, to)
	}
}

func (e *DeepSeekExecutor) ConvertResponse(body []byte, from, to translator.Format) ([]byte, error) {
	if from == to {
		return body, nil
	}

	switch {
	case from == translator.FormatOpenAI && to == translator.FormatClaude:
		return dsOpenAIToClaudeResponse(body)
	case from == translator.FormatClaude && to == translator.FormatOpenAI:
		return dsClaudeToOpenAIResponse(body)
	case from == translator.FormatOpenAI && to == translator.FormatOpenAIResponses:
		return OpenAIToResponsesResponse(body)
	case from == translator.FormatClaude && to == translator.FormatOpenAIResponses:
		return ClaudeToResponsesResponse(body)
	default:
		return nil, fmt.Errorf("deepseek: unsupported response conversion %s→%s", from, to)
	}
}

func (e *DeepSeekExecutor) RequestCustomize(body []byte, info *RequestInfo) []byte {
	if info.ActualModelName != "" {
		body = replaceModelField(body, info.ActualModelName)
	}
	if info.IsStream && info.RelayMode == translator.RelayModeChatCompletions {
		body = injectStreamOptionsOpenAI(body)
	}
	body = dsInjectThinking(body, info)
	return body
}

func (e *DeepSeekExecutor) ResponseCustomize(body []byte, info *RequestInfo) []byte {
	return body
}

func (e *DeepSeekExecutor) NewResponseStream(from, to translator.Format) (ResponseStream, error) {
	if from == to {
		return nil, nil
	}

	switch {
	case from == translator.FormatClaude && to == translator.FormatOpenAI:
		return newClaudeToOpenAIStream(), nil
	case from == translator.FormatOpenAI && to == translator.FormatClaude:
		return newOpenAIToClaudeStream(), nil
	default:
		return nil, fmt.Errorf("deepseek: streaming conversion %s→%s not implemented", from, to)
	}
}

func (e *DeepSeekExecutor) DoRequest(info *RequestInfo, body io.Reader) (*http.Response, error) {
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
// DeepSeek-specific customizations
// ========================================================================

func dsInjectThinking(body []byte, info *RequestInfo) []byte {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(body, &raw); err != nil {
		return body
	}

	if info.ThinkingDisabled {
		raw["thinking"] = json.RawMessage(`{"type":"disabled"}`)
		delete(raw, "reasoning_effort")
	} else if info.ThinkingEnabled || info.ReasoningEffort != "" {
		if _, exists := raw["thinking"]; !exists {
			if info.ReasoningEffort != "" {
				effort := dsMapEffort(info.ReasoningEffort)
				raw["thinking"] = json.RawMessage(`{"type":"enabled"}`)
				raw["reasoning_effort"], _ = json.Marshal(effort)
			} else {
				raw["thinking"] = json.RawMessage(`{"type":"enabled"}`)
			}
		}
	}

	result, _ := json.Marshal(raw)
	return result
}

func dsMapEffort(effort string) string {
	switch effort {
	case "low", "medium", "minimal":
		return "high"
	case "xhigh", "max":
		return "max"
	default:
		return "high"
	}
}

// ========================================================================
// Claude → Responses (deepseek-specific)
// ========================================================================

func dsClaudeToResponsesRequest(body []byte) ([]byte, error) {
	var req translator.ClaudeRequest
	if err := json.Unmarshal(body, &req); err != nil {
		return nil, fmt.Errorf("claude→responses: unmarshal: %w", err)
	}

	r := translator.ResponsesRequest{
		Model: req.Model,
	}
	if req.MaxTokens != nil {
		r.MaxOutputTokens = dsUintPtr(*req.MaxTokens)
	}
	if req.Temperature != nil {
		r.Temperature = req.Temperature
	}
	if req.TopP != nil {
		r.TopP = req.TopP
	}
	if req.Stream != nil {
		r.Stream = req.Stream
	}

	if req.System != nil {
		sysStr := dsExtractTextContent(req.System)
		if sysStr != "" {
			r.Instructions = json.RawMessage(`"` + sysStr + `"`)
		}
	}

	inputItems := make([]translator.InputItem, 0, len(req.Messages))
	for _, msg := range req.Messages {
		switch msg.Role {
		case "user":
			text := dsExtractTextContent(msg.Content)
			if text != "" {
				inputItems = append(inputItems, translator.InputItem{
					Type: "message", Role: "user", Content: text,
				})
			}
		case "assistant":
			text := dsExtractTextContent(msg.Content)
			if text != "" {
				inputItems = append(inputItems, translator.InputItem{
					Type: "message", Role: "assistant", Content: text,
				})
			}
			if blocks, ok := msg.Content.([]translator.ClaudeContentBlock); ok {
				for _, block := range blocks {
					if block.Type == "tool_use" {
						args, _ := json.Marshal(block.Input)
						inputItems = append(inputItems, translator.InputItem{
							Type: "message", Role: "assistant",
							Content: fmt.Sprintf(`[{"type":"output_text","text":"Tool: %s"}]`, block.Name),
						})
						inputItems = append(inputItems, translator.InputItem{
							Type: "function_call_output", CallID: block.ID, Output: json.RawMessage(args),
						})
					}
				}
			}
		}
	}

	r.Input, _ = json.Marshal(inputItems)
	return json.Marshal(r)
}

// ========================================================================
// OpenAI → Claude Request (deepseek-specific, ds prefix)
// ========================================================================

func dsOpenAIToClaudeRequest(body []byte) ([]byte, error) {
	var req translator.ChatRequest
	if err := json.Unmarshal(body, &req); err != nil {
		return nil, fmt.Errorf("openai→claude: unmarshal request: %w", err)
	}

	claude := translator.ClaudeRequest{
		Model:     req.Model,
		MaxTokens: dsUintPtr(4096),
	}

	if req.MaxCompletionTokens != nil && *req.MaxCompletionTokens > 0 {
		claude.MaxTokens = dsUintPtr(uint(*req.MaxCompletionTokens))
	} else if req.MaxTokens != nil && *req.MaxTokens > 0 {
		claude.MaxTokens = dsUintPtr(uint(*req.MaxTokens))
	}

	if req.Temperature != nil {
		claude.Temperature = req.Temperature
	}
	if req.TopP != nil {
		claude.TopP = req.TopP
	}
	if req.Stream != nil {
		claude.Stream = req.Stream
	}

	var systemParts []string
	claudeMsgs := make([]translator.ClaudeMessage, 0, len(req.Messages))

	for _, msg := range req.Messages {
		switch msg.Role {
		case translator.RoleSystem, translator.RoleDeveloper:
			systemParts = append(systemParts, dsExtractTextContent(msg.Content))
			continue
		case translator.RoleUser:
			claudeMsgs = append(claudeMsgs, translator.ClaudeMessage{
				Role: "user", Content: dsConvertOpenAIContentToClaude(msg.Content),
			})
		case translator.RoleAssistant:
			cm := translator.ClaudeMessage{Role: "assistant"}
			if msg.ReasoningContent != nil && *msg.ReasoningContent != "" {
				blocks := []translator.ClaudeContentBlock{
					{Type: "thinking", Thinking: dsStrPtr(*msg.ReasoningContent)},
				}
				if content, ok := msg.Content.(string); ok && content != "" {
					blocks = append(blocks, translator.ClaudeContentBlock{Type: "text", Text: dsStrPtr(content)})
				}
				if len(msg.ToolCalls) > 0 {
					for _, tc := range msg.ToolCalls {
						blocks = append(blocks, dsClaudeToolUseFromOpenAI(tc))
					}
				}
				cm.Content = blocks
			} else if len(msg.ToolCalls) > 0 {
				blocks := make([]translator.ClaudeContentBlock, 0, len(msg.ToolCalls)+1)
				if content, ok := msg.Content.(string); ok && content != "" {
					blocks = append(blocks, translator.ClaudeContentBlock{Type: "text", Text: dsStrPtr(content)})
				}
				for _, tc := range msg.ToolCalls {
					blocks = append(blocks, dsClaudeToolUseFromOpenAI(tc))
				}
				cm.Content = blocks
			} else {
				cm.Content = dsExtractTextContent(msg.Content)
			}
			claudeMsgs = append(claudeMsgs, cm)
		case translator.RoleTool:
			claudeMsgs = append(claudeMsgs, translator.ClaudeMessage{
				Role: "user",
				Content: []translator.ClaudeContentBlock{{
					Type: "tool_result", ToolUseID: msg.ToolCallID,
					Content: dsExtractTextContent(msg.Content),
				}},
			})
		}
	}

	if len(systemParts) > 0 {
		claude.System = strings.Join(systemParts, "\n")
	}

	if len(req.Tools) > 0 {
		var tools []translator.ClaudeTool
		if err := json.Unmarshal(req.Tools, &tools); err == nil {
			claude.Tools = tools
		} else {
			var otools []translator.Tool
			if err := json.Unmarshal(req.Tools, &otools); err == nil {
				claude.Tools = make([]translator.ClaudeTool, 0, len(otools))
				for _, ot := range otools {
					claude.Tools = append(claude.Tools, translator.ClaudeTool{
						Name: ot.Function.Name, Description: ot.Function.Description,
						InputSchema: ot.Function.Parameters, Type: "custom",
					})
				}
			} else {
				var raw []map[string]any
				if err := json.Unmarshal(req.Tools, &raw); err == nil {
					claude.Tools = dsToolListOpenAIToClaude(raw)
				}
			}
		}
	}

	if req.ToolChoice != nil {
		claude.ToolChoice = dsMapToolChoiceOpenAIToClaude(req.ToolChoice)
	}
	if req.ReasoningEffort != "" {
		claude.Thinking = &translator.ClaudeThinking{
			Type: "enabled", BudgetTokens: dsIntPtr(10000),
		}
	}

	claude.Messages = claudeMsgs
	return json.Marshal(claude)
}

// ========================================================================
// Claude → OpenAI Request (deepseek-specific, ds prefix)
// ========================================================================

func dsClaudeToOpenAIRequest(body []byte) ([]byte, error) {
	var req translator.ClaudeRequest
	if err := json.Unmarshal(body, &req); err != nil {
		return nil, fmt.Errorf("claude→openai: unmarshal request: %w", err)
	}

	openai := translator.ChatRequest{
		Model: req.Model,
	}
	if req.MaxTokens != nil {
		openai.MaxTokens = dsIntPtr(int(*req.MaxTokens))
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
		systemText := dsExtractTextContent(req.System)
		if systemText != "" {
			openai.Messages = append(openai.Messages, translator.Message{
				Role: "system", Content: systemText,
			})
		}
	}

	for _, cm := range req.Messages {
		om := translator.Message{Role: dsMapClaudeRoleToOpenAI(cm.Role)}
		switch content := cm.Content.(type) {
		case string:
			om.Content = content
		case []translator.ClaudeContentBlock:
			om.Content = dsConvertClaudeContentToOpenAI(content)
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
		openai.ToolChoice = dsMapToolChoiceClaudeToOpenAI(req.ToolChoice)
	}
	if req.Thinking != nil && req.Thinking.Type == "enabled" {
		openai.ReasoningEffort = "medium"
	}
	return json.Marshal(openai)
}

// ========================================================================
// OpenAI → Claude Response (deepseek-specific, ds prefix)
// ========================================================================

func dsOpenAIToClaudeResponse(body []byte) ([]byte, error) {
	var resp translator.ChatResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("openai→claude: unmarshal response: %w", err)
	}

	claudeResp := translator.ClaudeResponse{
		ID: "", Type: "message", Role: "assistant", Model: resp.Model,
	}

	for _, choice := range resp.Choices {
		blocks := make([]translator.ClaudeContentBlock, 0)
		if choice.Message.ReasoningContent != nil && *choice.Message.ReasoningContent != "" {
			blocks = append(blocks, translator.ClaudeContentBlock{
				Type: "thinking", Thinking: dsStrPtr(*choice.Message.ReasoningContent),
			})
		}
		if choice.Message.Content != nil {
			blocks = append(blocks, translator.ClaudeContentBlock{
				Type: "text", Text: choice.Message.Content,
			})
		}
		for _, tc := range choice.Message.ToolCalls {
			blocks = append(blocks, dsClaudeToolUseFromOpenAI(tc))
		}
		claudeResp.Content = blocks
		claudeResp.StopReason = dsMapFinishReasonToClaude(choice.FinishReason)
	}

	if resp.Usage != nil {
		claudeResp.Usage = &translator.ClaudeUsage{
			InputTokens: resp.Usage.PromptTokens, OutputTokens: resp.Usage.CompletionTokens,
		}
	}
	if resp.ID != "" {
		claudeResp.ID = "msg_" + resp.ID
	}
	return json.Marshal(claudeResp)
}

// ========================================================================
// Claude → OpenAI Response (deepseek-specific, ds prefix)
// ========================================================================

func dsClaudeToOpenAIResponse(body []byte) ([]byte, error) {
	var resp translator.ClaudeResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("claude→openai: unmarshal response: %w", err)
	}

	chatResp := translator.ChatResponse{
		ID: resp.ID, Object: "chat.completion",
		Created: time.Now().Unix(), Model: resp.Model,
	}

	msg := translator.ChatMessage{Role: "assistant"}
	var reasoningContent string
	var toolCalls []translator.ToolCall

	for _, block := range resp.Content {
		switch block.Type {
		case "text":
			if block.Text != nil {
				msg.Content = block.Text
			}
		case "thinking":
			reasoningContent = dsSafeStr(block.Thinking)
		case "tool_use":
			inputStr, _ := json.Marshal(block.Input)
			toolCalls = append(toolCalls, translator.ToolCall{
				ID: block.ID, Type: "function",
				Function: translator.FunctionCall{
					Name: block.Name, Arguments: string(inputStr),
				},
			})
		}
	}

	if reasoningContent != "" {
		msg.ReasoningContent = &reasoningContent
	}
	if len(toolCalls) > 0 {
		msg.ToolCalls = toolCalls
	}
	if msg.Content == nil || *msg.Content == "" {
		if len(toolCalls) == 0 {
			msg.Content = dsStrPtr("")
		}
	}

	finishReason := dsMapClaudeStopToOpenAI(resp.StopReason)
	chatResp.Choices = []translator.Choice{{
		Index: 0, Message: msg, FinishReason: finishReason,
	}}

	if resp.Usage != nil {
		chatResp.Usage = &translator.ChatUsage{
			PromptTokens: resp.Usage.InputTokens,
			CompletionTokens: resp.Usage.OutputTokens,
			TotalTokens: resp.Usage.InputTokens + resp.Usage.OutputTokens,
		}
	}

	return json.Marshal(chatResp)
}

// ========================================================================
// Internal helpers (ds-prefixed, deepseek-specific)
// ========================================================================

func dsExtractTextContent(content any) string {
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

func dsConvertOpenAIContentToClaude(content any) any {
	if content == nil {
		return ""
	}
	switch v := content.(type) {
	case string:
		if v == "" {
			return ""
		}
		return []translator.ClaudeContentBlock{{Type: "text", Text: &v}}
	case []translator.ContentPart:
		blocks := make([]translator.ClaudeContentBlock, 0, len(v))
		for _, part := range v {
			switch part.Type {
			case "text":
				blocks = append(blocks, translator.ClaudeContentBlock{Type: "text", Text: dsStrPtr(part.Text)})
			case "image_url":
				if part.ImageURL != nil {
					blocks = append(blocks, translator.ClaudeContentBlock{
						Type: "image",
						Source: &translator.ClaudeSource{Type: "url", URL: part.ImageURL.URL},
					})
				}
			case "input_audio":
				blocks = append(blocks, translator.ClaudeContentBlock{Type: "text", Text: dsStrPtr("[audio input]")})
			}
		}
		return blocks
	default:
		return fmt.Sprintf("%v", v)
	}
}

func dsConvertClaudeContentToOpenAI(content []translator.ClaudeContentBlock) any {
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

func dsClaudeToolUseFromOpenAI(tc translator.ToolCall) translator.ClaudeContentBlock {
	var input any
	if err := json.Unmarshal([]byte(tc.Function.Arguments), &input); err != nil {
		input = tc.Function.Arguments
	}
	return translator.ClaudeContentBlock{
		Type: "tool_use", ID: tc.ID, Name: tc.Function.Name, Input: input,
	}
}

func dsToolListOpenAIToClaude(raw []map[string]any) []translator.ClaudeTool {
	tools := make([]translator.ClaudeTool, 0, len(raw))
	for _, t := range raw {
		ct := translator.ClaudeTool{Type: "custom"}
		if fn, ok := t["function"].(map[string]any); ok {
			if name, ok := fn["name"].(string); ok {
				ct.Name = name
			}
			if desc, ok := fn["description"].(string); ok {
				ct.Description = desc
			}
			if params, ok := fn["parameters"]; ok {
				ct.InputSchema = params
			}
		}
		tools = append(tools, ct)
	}
	return tools
}

func dsMapToolChoiceOpenAIToClaude(tc any) any {
	switch v := tc.(type) {
	case string:
		switch v {
		case "auto", "required":
			return map[string]any{"type": "any"}
		case "none":
			return map[string]any{"type": "none"}
		default:
			return map[string]any{"type": "auto"}
		}
	case map[string]any:
		if fn, ok := v["function"].(map[string]any); ok {
			if name, ok := fn["name"].(string); ok {
				return translator.ClaudeToolChoice{Type: "tool", Name: name}
			}
		}
		return v
	case translator.ToolChoice:
		if v.Type == "tool" && v.Name != "" {
			return translator.ClaudeToolChoice{Type: "tool", Name: v.Name}
		}
		return map[string]any{"type": v.Type}
	default:
		return map[string]any{"type": "auto"}
	}
}

func dsMapToolChoiceClaudeToOpenAI(tc any) any {
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

func dsMapClaudeRoleToOpenAI(role string) string {
	switch role {
	case "user", "assistant":
		return role
	default:
		return "user"
	}
}

func dsMapFinishReasonToClaude(reason string) *string {
	switch reason {
	case translator.FinishReasonStop:
		return dsStrPtr("end_turn")
	case translator.FinishReasonToolCalls:
		return dsStrPtr("tool_use")
	case translator.FinishReasonLength:
		return dsStrPtr("max_tokens")
	default:
		return dsStrPtr(reason)
	}
}

func dsMapClaudeStopToOpenAI(stop *string) string {
	if stop == nil {
		return ""
	}
	switch *stop {
	case "end_turn":
		return "stop"
	case "tool_use":
		return "tool_calls"
	case "max_tokens":
		return "length"
	case "stop_sequence":
		return "stop"
	default:
		return *stop
	}
}

func dsSafeStr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func dsStrPtr(s string) *string {
	return &s
}

func dsIntPtr(i int) *int {
	return &i
}

func dsUintPtr(u uint) *uint {
	return &u
}
