package executor

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/just4zeroq/Omni-link/translator"
)

func init() {
	Register("claude", &ClaudeExecutor{})
}

// ClaudeExecutor handles Anthropic Claude endpoints.
// Native format: claude (passthrough).
type ClaudeExecutor struct {
	channel any
}

func (e *ClaudeExecutor) Init(channel any) {
	e.channel = channel
}

func (e *ClaudeExecutor) GetName() string {
	if ch, ok := e.channel.(interface{ GetName() string }); ok {
		return ch.GetName()
	}
	return "Claude"
}

func (e *ClaudeExecutor) NativeFormats() []EndpointCapability {
	return []EndpointCapability{
		{Format: translator.FormatClaude, RelayMode: translator.RelayModeClaudeMessages},
	}
}

func (e *ClaudeExecutor) GetRequestURL(info *RequestInfo) (string, error) {
	baseURL := info.BaseURL
	if baseURL == "" {
		baseURL = "https://api.anthropic.com/v1"
	}
	baseURL = strings.TrimSuffix(baseURL, "/")
	return baseURL + "/messages", nil
}

func (e *ClaudeExecutor) SetupRequestHeader(header http.Header, info *RequestInfo) error {
	header.Set("x-api-key", info.ApiKey)
	header.Set("anthropic-version", "2023-06-01")
	header.Set("Content-Type", "application/json")
	if info.IsStream {
		header.Set("Accept", "text/event-stream")
	}
	return nil
}

func (e *ClaudeExecutor) ConvertRequest(body []byte, from, to translator.Format) ([]byte, error) {
	if from == to {
		return body, nil
	}

	switch {
	case from == translator.FormatOpenAI && to == translator.FormatClaude:
		return clOpenAIToClaudeRequest(body)
	case from == translator.FormatOpenAIResponses && to == translator.FormatClaude:
		return ResponsesToClaudeRequest(body)
	default:
		return nil, fmt.Errorf("claude: unsupported request conversion %s→%s", from, to)
	}
}

func (e *ClaudeExecutor) ConvertResponse(body []byte, from, to translator.Format) ([]byte, error) {
	if from == to {
		return body, nil
	}

	switch {
	case from == translator.FormatClaude && to == translator.FormatOpenAI:
		return clClaudeToOpenAIResponse(body)
	case from == translator.FormatClaude && to == translator.FormatOpenAIResponses:
		return ClaudeToResponsesResponse(body)
	default:
		return nil, fmt.Errorf("claude: unsupported response conversion %s→%s", from, to)
	}
}

func (e *ClaudeExecutor) RequestCustomize(body []byte, info *RequestInfo) []byte {
	if info.ActualModelName != "" {
		body = replaceModelField(body, info.ActualModelName)
	}
	return body
}

func (e *ClaudeExecutor) ResponseCustomize(body []byte, info *RequestInfo) []byte {
	return body
}

func (e *ClaudeExecutor) NewResponseStream(from, to translator.Format) (ResponseStream, error) {
	if from == to {
		return nil, nil
	}

	switch {
	case from == translator.FormatClaude && to == translator.FormatOpenAI:
		return newClaudeToOpenAIStream(), nil
	case from == translator.FormatClaude && to == translator.FormatOpenAIResponses:
		// Claude → Responses streaming not yet implemented
		return nil, nil
	case from == translator.FormatOpenAI && to == translator.FormatClaude:
		return newOpenAIToClaudeStream(), nil
	default:
		return nil, fmt.Errorf("claude: streaming conversion %s→%s not implemented", from, to)
	}
}

func (e *ClaudeExecutor) DoRequest(info *RequestInfo, body io.Reader) (*http.Response, error) {
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
// Responses API ↔ Claude (claude-only, no conflicts)
// ========================================================================

// ResponsesToClaudeRequest converts Responses API request directly to Claude format.
func ResponsesToClaudeRequest(body []byte) ([]byte, error) {
	var req translator.ResponsesRequest
	if err := json.Unmarshal(body, &req); err != nil {
		return nil, fmt.Errorf("responses→claude: unmarshal: %w", err)
	}

	claude := translator.ClaudeRequest{
		Model: req.Model,
	}
	if req.MaxOutputTokens != nil {
		claude.MaxTokens = clUintPtr(*req.MaxOutputTokens)
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

	if len(req.Instructions) > 0 {
		var instr string
		if err := json.Unmarshal(req.Instructions, &instr); err == nil && instr != "" {
			claude.System = instr
		}
	}

	claude.Messages = make([]translator.ClaudeMessage, 0)
	if len(req.Input) > 0 {
		msgs, err := parseResponsesInputToClaude(req.Input)
		if err == nil {
			claude.Messages = msgs
		}
	}

	return json.Marshal(claude)
}

// ClaudeToResponsesResponse converts Claude response directly to Responses API format.
func ClaudeToResponsesResponse(body []byte) ([]byte, error) {
	var resp translator.ClaudeResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("claude→responses: unmarshal: %w", err)
	}

	r := translator.ResponsesResponse{
		ID:     resp.ID,
		Object: "response",
		Status: "completed",
		Model:  resp.Model,
	}

	var textContent string
	var toolCalls []translator.ResponsesOutput
	for _, block := range resp.Content {
		switch block.Type {
		case "text":
			if block.Text != nil {
				textContent += *block.Text
			}
		case "tool_use":
			args, _ := json.Marshal(block.Input)
			toolCalls = append(toolCalls, translator.ResponsesOutput{
				Type: "function_call", ID: block.ID, CallID: block.ID,
				Name: block.Name, Arguments: string(args), Status: "completed",
			})
		}
	}

	if textContent != "" {
		r.Output = append(r.Output, translator.ResponsesOutput{
			Type: "message", ID: "msg_" + resp.ID, Status: "completed", Role: "assistant",
			Content: []translator.ResponsesOutputContent{{Type: "output_text", Text: textContent}},
		})
	}
	r.Output = append(r.Output, toolCalls...)

	if resp.Usage != nil {
		r.Usage = &translator.ResponsesUsage{
			InputTokens:  resp.Usage.InputTokens,
			OutputTokens: resp.Usage.OutputTokens,
			TotalTokens:  resp.Usage.InputTokens + resp.Usage.OutputTokens,
		}
	}

	r.OutputText = textContent
	return json.Marshal(r)
}

func parseResponsesInputToClaude(input json.RawMessage) ([]translator.ClaudeMessage, error) {
	var str string
	if err := json.Unmarshal(input, &str); err == nil {
		return []translator.ClaudeMessage{{Role: "user", Content: str}}, nil
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
			}
			if text, ok := r["text"].(string); ok {
				item.Text = text
			}
			if c, ok := r["content"].(string); ok {
				item.Content = c
			}
			if callID, ok := r["call_id"].(string); ok {
				item.CallID = callID
			}
			if output, ok := r["output"]; ok {
				item.Output = output
			}
			items = append(items, item)
		}
	}

	msgs := make([]translator.ClaudeMessage, 0, len(items))
	for _, item := range items {
		switch item.Type {
		case "input_text", "":
			text := item.Text
			if text == "" {
				text = item.Content
			}
			msgs = append(msgs, translator.ClaudeMessage{
				Role: "user",
				Content: []translator.ClaudeContentBlock{{Type: "text", Text: &text}},
			})
		case "message":
			role := item.Role
			if role == "" {
				role = "user"
			}
			content := item.Content
			if content == "" {
				content = item.Text
			}
			msgs = append(msgs, translator.ClaudeMessage{
				Role: role,
				Content: []translator.ClaudeContentBlock{{Type: "text", Text: &content}},
			})
		case "function_call_output":
			outputStr, _ := json.Marshal(item.Output)
			msgs = append(msgs, translator.ClaudeMessage{
				Role: "user",
				Content: []translator.ClaudeContentBlock{{
					Type: "tool_result", ToolUseID: item.CallID, Content: string(outputStr),
				}},
			})
		case "item_reference":
			continue
		}
	}
	return msgs, nil
}

func bodyFromBytes(b []byte) io.Reader {
	return bytes.NewReader(b)
}

// ========================================================================
// Claude ↔ OpenAI conversions (claude-specific, cl prefix)
// ========================================================================

func clOpenAIToClaudeRequest(body []byte) ([]byte, error) {
	var req translator.ChatRequest
	if err := json.Unmarshal(body, &req); err != nil {
		return nil, fmt.Errorf("openai→claude: unmarshal request: %w", err)
	}

	claude := translator.ClaudeRequest{
		Model:     req.Model,
		MaxTokens: clUintPtr(4096),
	}

	if req.MaxCompletionTokens != nil && *req.MaxCompletionTokens > 0 {
		claude.MaxTokens = clUintPtr(uint(*req.MaxCompletionTokens))
	} else if req.MaxTokens != nil && *req.MaxTokens > 0 {
		claude.MaxTokens = clUintPtr(uint(*req.MaxTokens))
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
			systemParts = append(systemParts, clExtractTextContent(msg.Content))
			continue
		case translator.RoleUser:
			claudeMsgs = append(claudeMsgs, translator.ClaudeMessage{
				Role: "user", Content: clConvertOpenAIContentToClaude(msg.Content),
			})
		case translator.RoleAssistant:
			cm := translator.ClaudeMessage{Role: "assistant"}
			if msg.ReasoningContent != nil && *msg.ReasoningContent != "" {
				blocks := []translator.ClaudeContentBlock{
					{Type: "thinking", Thinking: clStrPtr(*msg.ReasoningContent)},
				}
				if content, ok := msg.Content.(string); ok && content != "" {
					blocks = append(blocks, translator.ClaudeContentBlock{Type: "text", Text: clStrPtr(content)})
				}
				if len(msg.ToolCalls) > 0 {
					for _, tc := range msg.ToolCalls {
						blocks = append(blocks, clClaudeToolUseFromOpenAI(tc))
					}
				}
				cm.Content = blocks
			} else if len(msg.ToolCalls) > 0 {
				blocks := make([]translator.ClaudeContentBlock, 0, len(msg.ToolCalls)+1)
				if content, ok := msg.Content.(string); ok && content != "" {
					blocks = append(blocks, translator.ClaudeContentBlock{Type: "text", Text: clStrPtr(content)})
				}
				for _, tc := range msg.ToolCalls {
					blocks = append(blocks, clClaudeToolUseFromOpenAI(tc))
				}
				cm.Content = blocks
			} else {
				cm.Content = clExtractTextContent(msg.Content)
			}
			claudeMsgs = append(claudeMsgs, cm)
		case translator.RoleTool:
			claudeMsgs = append(claudeMsgs, translator.ClaudeMessage{
				Role: "user",
				Content: []translator.ClaudeContentBlock{{
					Type: "tool_result", ToolUseID: msg.ToolCallID,
					Content: clExtractTextContent(msg.Content),
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
					claude.Tools = clToolListOpenAIToClaude(raw)
				}
			}
		}
	}

	if req.ToolChoice != nil {
		claude.ToolChoice = clMapToolChoiceOpenAIToClaude(req.ToolChoice)
	}
	if req.ReasoningEffort != "" {
		claude.Thinking = &translator.ClaudeThinking{
			Type: "enabled", BudgetTokens: clIntPtr(10000),
		}
	}

	claude.Messages = claudeMsgs
	return json.Marshal(claude)
}

func clClaudeToOpenAIResponse(body []byte) ([]byte, error) {
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
			reasoningContent = clSafeStr(block.Thinking)
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
			msg.Content = clStrPtr("")
		}
	}

	finishReason := clMapClaudeStopToOpenAI(resp.StopReason)
	chatResp.Choices = []translator.Choice{{
		Index: 0, Message: msg, FinishReason: finishReason,
	}}

	if resp.Usage != nil {
		chatResp.Usage = &translator.ChatUsage{
			PromptTokens:     resp.Usage.InputTokens,
			CompletionTokens: resp.Usage.OutputTokens,
			TotalTokens:      resp.Usage.InputTokens + resp.Usage.OutputTokens,
		}
	}

	return json.Marshal(chatResp)
}

// ========================================================================
// Internal helpers (cl-prefixed, claude-specific)
// ========================================================================

func clExtractTextContent(content any) string {
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

func clConvertOpenAIContentToClaude(content any) any {
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
				blocks = append(blocks, translator.ClaudeContentBlock{Type: "text", Text: clStrPtr(part.Text)})
			case "image_url":
				if part.ImageURL != nil {
					blocks = append(blocks, translator.ClaudeContentBlock{
						Type: "image",
						Source: &translator.ClaudeSource{Type: "url", URL: part.ImageURL.URL},
					})
				}
			case "input_audio":
				blocks = append(blocks, translator.ClaudeContentBlock{Type: "text", Text: clStrPtr("[audio input]")})
			}
		}
		return blocks
	default:
		return fmt.Sprintf("%v", v)
	}
}

func clClaudeToolUseFromOpenAI(tc translator.ToolCall) translator.ClaudeContentBlock {
	var input any
	if err := json.Unmarshal([]byte(tc.Function.Arguments), &input); err != nil {
		input = tc.Function.Arguments
	}
	return translator.ClaudeContentBlock{
		Type: "tool_use", ID: tc.ID, Name: tc.Function.Name, Input: input,
	}
}

func clToolListOpenAIToClaude(raw []map[string]any) []translator.ClaudeTool {
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

func clMapToolChoiceOpenAIToClaude(tc any) any {
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

func clMapClaudeStopToOpenAI(stop *string) string {
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

func clSafeStr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func clStrPtr(s string) *string {
	return &s
}

func clIntPtr(i int) *int {
	return &i
}

func clUintPtr(u uint) *uint {
	return &u
}

// ========================================================================
// ResponseStream: Claude SSE ↔ OpenAI Chat SSE
// ========================================================================

// claudeToOpenAIStream converts Claude SSE events to OpenAI Chat format SSE.
// Maintains state for content block index tracking and usage accumulation.
type claudeToOpenAIStream struct {
	buf       bytes.Buffer // incomplete event buffer
	blockIdx  int          // current content block index
	msgID     string
	model     string
	usage     *translator.Usage
	finished  bool
}

func newClaudeToOpenAIStream() *claudeToOpenAIStream {
	return &claudeToOpenAIStream{blockIdx: -1}
}

func (s *claudeToOpenAIStream) Feed(chunk []byte) ([]byte, error) {
	s.buf.Write(chunk)
	if !bytes.Contains(s.buf.Bytes(), []byte("\n\n")) {
		return nil, nil // wait for complete event
	}

	// Split buffer into complete events
	data := s.buf.Bytes()
	s.buf.Reset()

	var out []byte
	for {
		idx := bytes.Index(data, []byte("\n\n"))
		if idx < 0 {
			s.buf.Write(data) // incomplete, buffer for next Feed
			break
		}

		event := data[:idx]
		data = data[idx+2:]

		converted, err := s.convertEvent(event)
		if err != nil {
			continue // skip malformed events
		}
		if len(converted) > 0 {
			out = append(out, converted...)
		}
	}

	return out, nil
}

func (s *claudeToOpenAIStream) End() ([]byte, error) {
	if s.finished {
		return nil, nil
	}
	s.finished = true

	// Flush remaining buffer
	if s.buf.Len() > 0 {
		converted, _ := s.convertEvent(s.buf.Bytes())
		s.buf.Reset()
		if len(converted) > 0 {
			return append(converted, []byte("data: [DONE]\n\n")...), nil
		}
	}
	return []byte("data: [DONE]\n\n"), nil
}

func (s *claudeToOpenAIStream) Usage() *translator.Usage {
	return s.usage
}

func (s *claudeToOpenAIStream) convertEvent(event []byte) ([]byte, error) {
	// Parse event type from "event: xxx" line
	eventType := parseSSEEventType(event)
	if eventType == "" {
		return nil, fmt.Errorf("no event type")
	}

	// Parse data from "data: xxx" line
	dataField := parseSSEDataField(event)
	if dataField == nil && eventType != "message_stop" {
		return nil, fmt.Errorf("no data field")
	}

	switch eventType {
	case "message_start":
		return s.onMessageStart(dataField)

	case "content_block_start":
		return s.onContentBlockStart(dataField)

	case "content_block_delta":
		return s.onContentBlockDelta(dataField)

	case "content_block_stop":
		return s.onContentBlockStop()

	case "message_delta":
		return s.onMessageDelta(dataField)

	case "message_stop":
		return s.onMessageStop()

	case "ping":
		return nil, nil // skip

	default:
		return nil, nil // unknown events pass through as empty
	}
}

func (s *claudeToOpenAIStream) onMessageStart(data []byte) ([]byte, error) {
	var msg struct {
		Type    string `json:"type"`
		Message struct {
			ID    string `json:"id"`
			Model string `json:"model"`
			Role  string `json:"role"`
		} `json:"message"`
	}
	if err := json.Unmarshal(data, &msg); err != nil {
		return nil, err
	}
	s.msgID = msg.Message.ID
	s.model = msg.Message.Model
	return nil, nil // message_start emits nothing to client
}

func (s *claudeToOpenAIStream) onContentBlockStart(data []byte) ([]byte, error) {
	var block struct {
		Type         string `json:"type"`
		Index        int    `json:"index"`
		ContentBlock struct {
			Type string `json:"type"`
			Name string `json:"name,omitempty"`
			ID   string `json:"id,omitempty"`
		} `json:"content_block"`
	}
	if err := json.Unmarshal(data, &block); err != nil {
		return nil, err
	}

	s.blockIdx = block.Index

	switch block.ContentBlock.Type {
	case "text":
		// Emit role chunk to start
		chunk := map[string]any{
			"choices": []map[string]any{{
				"index": block.Index,
				"delta": map[string]any{
					"role":    "assistant",
					"content": "",
				},
			}},
		}
		return formatOpenAIChunk(chunk), nil

	case "tool_use":
		// Emit tool call start
		chunk := map[string]any{
			"choices": []map[string]any{{
				"index": block.Index,
				"delta": map[string]any{
					"role":    "assistant",
					"content": "",
					"tool_calls": []map[string]any{{
						"index": block.Index,
						"id":    block.ContentBlock.ID,
						"type":  "function",
						"function": map[string]any{
							"name":      block.ContentBlock.Name,
							"arguments": "",
						},
					}},
				},
			}},
		}
		return formatOpenAIChunk(chunk), nil

	default:
		return nil, nil
	}
}

func (s *claudeToOpenAIStream) onContentBlockDelta(data []byte) ([]byte, error) {
	var delta struct {
		Type  string `json:"type"`
		Index int    `json:"index"`
		Delta struct {
			Type     string `json:"type"`
			Text     string `json:"text,omitempty"`
			Partial  string `json:"partial_json,omitempty"`
		} `json:"delta"`
	}
	if err := json.Unmarshal(data, &delta); err != nil {
		return nil, err
	}

	switch delta.Delta.Type {
	case "text_delta":
		chunk := map[string]any{
			"choices": []map[string]any{{
				"index": delta.Index,
				"delta": map[string]any{
					"content": delta.Delta.Text,
				},
			}},
		}
		return formatOpenAIChunk(chunk), nil

	case "input_json_delta":
		chunk := map[string]any{
			"choices": []map[string]any{{
				"index": delta.Index,
				"delta": map[string]any{
					"tool_calls": []map[string]any{{
						"index": delta.Index,
						"function": map[string]any{
							"arguments": delta.Delta.Partial,
						},
					}},
				},
			}},
		}
		return formatOpenAIChunk(chunk), nil

	default:
		return nil, nil
	}
}

func (s *claudeToOpenAIStream) onContentBlockStop() ([]byte, error) {
	// Current block done. Emit empty chunk to signal progression.
	return nil, nil
}

func (s *claudeToOpenAIStream) onMessageDelta(data []byte) ([]byte, error) {
	var msg struct {
		Type  string `json:"type"`
		Delta struct {
			StopReason   string `json:"stop_reason"`
			StopSequence string `json:"stop_sequence"`
		} `json:"delta"`
		Usage struct {
			InputTokens  int `json:"input_tokens"`
			OutputTokens int `json:"output_tokens"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(data, &msg); err != nil {
		return nil, err
	}

	// Store usage
	if msg.Usage.OutputTokens > 0 || msg.Usage.InputTokens > 0 {
		s.usage = &translator.Usage{
			PromptTokens:     msg.Usage.InputTokens,
			CompletionTokens: msg.Usage.OutputTokens,
		}
	}

	// Emit finish chunk
	finishReason := clMapClaudeStopToOpenAI(&msg.Delta.StopReason)
	chunk := map[string]any{
		"choices": []map[string]any{{
			"index":         0,
			"delta":         map[string]any{},
			"finish_reason": finishReason,
		}},
	}
	return formatOpenAIChunk(chunk), nil
}

func (s *claudeToOpenAIStream) onMessageStop() ([]byte, error) {
	s.finished = true
	return nil, nil // End() returns [DONE]
}

// ========================================================================
// openAIToClaudeStream — OpenAI Chat SSE → Claude SSE
// ========================================================================

// openAIToClaudeStream converts OpenAI Chat SSE chunks to Claude Messages SSE format.
type openAIToClaudeStream struct {
	buf         bytes.Buffer
	msgID       string
	blockIdx    int
	hasStarted  bool
	hasContent  bool
	usage       *translator.Usage
	finished    bool
}

func newOpenAIToClaudeStream() *openAIToClaudeStream {
	return &openAIToClaudeStream{}
}

func (s *openAIToClaudeStream) Feed(chunk []byte) ([]byte, error) {
	s.buf.Write(chunk)
	if !bytes.Contains(s.buf.Bytes(), []byte("\n\n")) {
		return nil, nil
	}

	data := s.buf.Bytes()
	s.buf.Reset()

	var out []byte
	for {
		idx := bytes.Index(data, []byte("\n\n"))
		if idx < 0 {
			s.buf.Write(data)
			break
		}

		event := data[:idx]
		data = data[idx+2:]

		converted, err := s.convertChunk(event)
		if err != nil {
			continue
		}
		if len(converted) > 0 {
			out = append(out, converted...)
		}
	}

	return out, nil
}

func (s *openAIToClaudeStream) End() ([]byte, error) {
	if s.finished {
		return nil, nil
	}
	s.finished = true

	// Flush remaining buffer
	if s.buf.Len() > 0 {
		s.convertChunk(s.buf.Bytes())
		s.buf.Reset()
	}

	// Close any open content block
	var out []byte
	if s.hasContent || s.hasStarted {
		out = append(out, formatClaudeEvent("content_block_stop", map[string]any{
			"type":  "content_block_stop",
			"index": s.blockIdx,
		})...)
	}

	// message_stop
	out = append(out, formatClaudeEvent("message_stop", map[string]any{
		"type": "message_stop",
	})...)

	return out, nil
}

func (s *openAIToClaudeStream) Usage() *translator.Usage {
	return s.usage
}

func (s *openAIToClaudeStream) convertChunk(line []byte) ([]byte, error) {
	// Strip "data: " prefix
	if !bytes.HasPrefix(line, []byte("data: ")) {
		return nil, nil // skip non-data lines
	}
	data := bytes.TrimPrefix(line, []byte("data: "))
	data = bytes.TrimSpace(data)

	// Check for [DONE]
	if string(data) == "[DONE]" {
		return nil, nil
	}

	// Parse OpenAI chat chunk
	var chunk struct {
		Choices []struct {
			Index        int    `json:"index"`
			Delta        struct {
				Role      string `json:"role"`
				Content   string `json:"content"`
				ToolCalls []struct {
					Index    int    `json:"index"`
					ID       string `json:"id"`
					Type     string `json:"type"`
					Function struct {
						Name      string `json:"name"`
						Arguments string `json:"arguments"`
					} `json:"function"`
				} `json:"tool_calls"`
			} `json:"delta"`
			FinishReason string `json:"finish_reason"`
		} `json:"choices"`
		Usage *struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
			TotalTokens      int `json:"total_tokens"`
		} `json:"usage"`
	}

	if err := json.Unmarshal(data, &chunk); err != nil {
		return nil, err
	}

	// Check for usage in stream_options
	if chunk.Usage != nil {
		s.usage = &translator.Usage{
			PromptTokens:     chunk.Usage.PromptTokens,
			CompletionTokens: chunk.Usage.CompletionTokens,
		}
	}

	if len(chunk.Choices) == 0 {
		return nil, nil
	}

	choice := chunk.Choices[0]
	var out []byte

	// message_start on first chunk with role
	if !s.hasStarted && choice.Delta.Role == "assistant" {
		s.hasStarted = true
		out = append(out, formatClaudeEvent("message_start", map[string]any{
			"type": "message_start",
			"message": map[string]any{
				"id":      "msg_" + randHex(8),
				"type":    "message",
				"role":    "assistant",
				"content": []any{},
				"model":   "",
			},
		})...)
	}

	// content_block_start for text
	if choice.Delta.Content != "" && !s.hasContent {
		s.hasContent = true
		s.blockIdx++

		out = append(out, formatClaudeEvent("content_block_start", map[string]any{
			"type":  "content_block_start",
			"index": s.blockIdx,
			"content_block": map[string]any{
				"type": "text",
				"text": "",
			},
		})...)

		// Also emit the first text as a delta
		if choice.Delta.Content != "" {
			out = append(out, formatClaudeEvent("content_block_delta", map[string]any{
				"type":  "content_block_delta",
				"index": s.blockIdx,
				"delta": map[string]any{
					"type": "text_delta",
					"text": choice.Delta.Content,
				},
			})...)
		}
		return out, nil
	}

	// content_block_delta for ongoing text
	if choice.Delta.Content != "" {
		out = append(out, formatClaudeEvent("content_block_delta", map[string]any{
			"type":  "content_block_delta",
			"index": s.blockIdx,
			"delta": map[string]any{
				"type": "text_delta",
				"text": choice.Delta.Content,
			},
		})...)
	}

	// tool_calls
	if len(choice.Delta.ToolCalls) > 0 {
		for _, tc := range choice.Delta.ToolCalls {
			if tc.ID != "" {
				s.blockIdx++
				out = append(out, formatClaudeEvent("content_block_start", map[string]any{
					"type":  "content_block_start",
					"index": s.blockIdx,
					"content_block": map[string]any{
						"type": "tool_use",
						"id":   tc.ID,
						"name": tc.Function.Name,
						"input": map[string]any{},
					},
				})...)
			}
			if tc.Function.Arguments != "" {
				out = append(out, formatClaudeEvent("content_block_delta", map[string]any{
					"type":  "content_block_delta",
					"index": s.blockIdx,
					"delta": map[string]any{
						"type":        "input_json_delta",
						"partial_json": tc.Function.Arguments,
					},
				})...)
			}
		}
	}

	// finish_reason → message_delta
	if choice.FinishReason != "" {
		stopReason := mapOpenAIFinishToClaude(choice.FinishReason)

		out = append(out, formatClaudeEvent("content_block_stop", map[string]any{
			"type":  "content_block_stop",
			"index": s.blockIdx,
		})...)

		out = append(out, formatClaudeEvent("message_delta", map[string]any{
			"type": "message_delta",
			"delta": map[string]any{
				"stop_reason":   stopReason,
				"stop_sequence": nil,
			},
		})...)
	}

	return out, nil
}

// ========================================================================
// SSE formatting helpers
// ========================================================================

func formatOpenAIChunk(data map[string]any) []byte {
	b, _ := json.Marshal(data)
	return []byte("data: " + string(b) + "\n\n")
}

func formatClaudeEvent(event string, data map[string]any) []byte {
	b, _ := json.Marshal(data)
	return []byte("event: " + event + "\ndata: " + string(b) + "\n\n")
}

func parseSSEEventType(event []byte) string {
	for _, line := range bytes.Split(event, []byte("\n")) {
		line = bytes.TrimSpace(line)
		if bytes.HasPrefix(line, []byte("event: ")) {
			return string(bytes.TrimPrefix(line, []byte("event: ")))
		}
	}
	return ""
}

func parseSSEDataField(event []byte) []byte {
	for _, line := range bytes.Split(event, []byte("\n")) {
		line = bytes.TrimSpace(line)
		if bytes.HasPrefix(line, []byte("data: ")) {
			return bytes.TrimSpace(bytes.TrimPrefix(line, []byte("data: ")))
		}
	}
	return nil
}

func mapOpenAIFinishToClaude(finish string) string {
	switch finish {
	case "stop":
		return "end_turn"
	case "tool_calls":
		return "tool_use"
	case "length":
		return "max_tokens"
	default:
		return finish
	}
}

// randHex generates a short hex string for message IDs.
func randHex(n int) string {
	const hexChars = "0123456789abcdef"
	b := make([]byte, n)
	for i := range b {
		b[i] = hexChars[int(streamRandState%16)]
		streamRandState = streamRandState*6364136223846793005 + 1442695040888963407
	}
	return string(b)
}

var streamRandState uint64 = 1442695040888963407
