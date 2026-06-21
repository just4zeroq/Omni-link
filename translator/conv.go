// Package translator — format conversion functions.
//
// This file implements the canonical conversion functions for all
// supported protocol format pairs. Conversion is always direct —
// no intermediate hub format.
//
// The functions here are the single source of truth for format conversion.
// Each executor's ConvertRequest/ConvertResponse delegates to these.
package translator

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// ========================================================================
// Request vs Response detection
// ========================================================================

// bodyKind distinguishes request from response JSON bodies.
type bodyKind int

const (
	bodyUnknown  bodyKind = iota
	bodyRequest           // has "messages" (openai/claude) or "input" (responses)
	bodyResponse          // has "choices" (openai) or "output" (responses) or "type":"message" (claude)
)

func detectBodyKind(body []byte) bodyKind {
	var raw map[string]any
	if err := json.Unmarshal(body, &raw); err != nil {
		return bodyUnknown
	}

	// Response discriminators
	if _, ok := raw["choices"]; ok {
		return bodyResponse // OpenAI Chat response
	}
	if _, ok := raw["output"]; ok {
		return bodyResponse // Responses API response
	}
	if t, _ := raw["type"].(string); t == "message" {
		if _, ok := raw["content"]; ok {
			return bodyResponse // Claude response
		}
	}

	// Request discriminators
	if _, ok := raw["messages"]; ok {
		return bodyRequest // OpenAI Chat or Claude request
	}
	if _, ok := raw["input"]; ok {
		return bodyRequest // Responses API request
	}
	if _, ok := raw["max_tokens"]; ok {
		return bodyRequest // Claude request (max_tokens without messages = partial)
	}

	return bodyUnknown
}

// isClaudeRequest checks if a known-request body is Claude-specific.
func isClaudeRequest(body []byte) bool {
	var raw map[string]any
	if err := json.Unmarshal(body, &raw); err != nil {
		return false
	}
	if model, ok := raw["model"].(string); ok {
		if strings.HasPrefix(model, "claude-") || strings.HasPrefix(model, "anthropic.") {
			return true
		}
	}
	if _, ok := raw["max_tokens"]; ok {
		if _, ok := raw["temperature"]; !ok {
			return true // Claude has max_tokens as required, no temperature = more likely Claude
		}
	}
	return false
}

// isResponsesRequest checks if a known-request body is Responses API.
func isResponsesRequest(body []byte) bool {
	var raw map[string]any
	if err := json.Unmarshal(body, &raw); err != nil {
		return false
	}
	if input, ok := raw["input"]; ok && input != nil {
		if _, hasMsgs := raw["messages"]; !hasMsgs {
			return true
		}
	}
	return false
}

// ========================================================================
// Claude → OpenAI
// ========================================================================

func claudeToOpenAI(body []byte) ([]byte, error) {
	kind := detectBodyKind(body)
	switch kind {
	case bodyRequest:
		return claudeToOpenAIRequest(body)
	case bodyResponse:
		return claudeToOpenAIResponse(body)
	default:
		return nil, fmt.Errorf("cannot detect body kind for claude→openai conversion")
	}
}

func claudeToOpenAIRequest(body []byte) ([]byte, error) {
	var req ClaudeRequest
	if err := json.Unmarshal(body, &req); err != nil {
		return nil, fmt.Errorf("claude→openai: unmarshal request: %w", err)
	}

	chatReq := ChatRequest{Model: req.Model}
	if req.MaxTokens != nil {
		chatReq.MaxTokens = ptrInt(int(*req.MaxTokens))
	}
	if req.Temperature != nil {
		chatReq.Temperature = req.Temperature
	}
	if req.TopP != nil {
		chatReq.TopP = req.TopP
	}
	if req.Stream != nil {
		chatReq.Stream = req.Stream
	}

	chatReq.Messages = make([]Message, 0)
	if req.System != nil {
		sysText := extractTextContent(req.System)
		if sysText != "" {
			chatReq.Messages = append(chatReq.Messages, Message{Role: RoleSystem, Content: sysText})
		}
	}

	for _, cm := range req.Messages {
		om := Message{Role: mapClaudeRoleToOpenAI(cm.Role)}
		switch content := cm.Content.(type) {
		case string:
			om.Content = content
		case []ClaudeContentBlock:
			om.Content = convertClaudeContentToOpenAI(content)
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
		chatReq.Messages = append(chatReq.Messages, om)
	}

	if len(req.Tools) > 0 {
		raw, _ := json.Marshal(req.Tools)
		chatReq.Tools = raw
	}
	if req.ToolChoice != nil {
		chatReq.ToolChoice = mapToolChoiceClaudeToOpenAI(req.ToolChoice)
	}
	if req.Thinking != nil && req.Thinking.Type == "enabled" {
		chatReq.ReasoningEffort = "medium"
	}

	return json.Marshal(chatReq)
}

func claudeToOpenAIResponse(body []byte) ([]byte, error) {
	var resp ClaudeResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("claude→openai: unmarshal response: %w", err)
	}

	chatResp := ChatResponse{
		ID:      resp.ID,
		Object:  "chat.completion",
		Created: time.Now().Unix(),
		Model:   resp.Model,
	}

	msg := ChatMessage{Role: "assistant"}
	var reasoningContent string
	var toolCalls []ToolCall

	for _, block := range resp.Content {
		switch block.Type {
		case "text":
			if block.Text != nil {
				msg.Content = block.Text
			}
		case "thinking":
			reasoningContent = safeStr(block.Thinking)
		case "tool_use":
			inputStr, _ := json.Marshal(block.Input)
			toolCalls = append(toolCalls, ToolCall{
				ID: block.ID, Type: "function",
				Function: FunctionCall{Name: block.Name, Arguments: string(inputStr)},
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
			msg.Content = ptrStr("")
		}
	}

	finishReason := mapClaudeStopToOpenAI(resp.StopReason)
	chatResp.Choices = []Choice{{
		Index: 0, Message: msg, FinishReason: finishReason,
	}}

	if resp.Usage != nil {
		chatResp.Usage = &ChatUsage{
			PromptTokens:     resp.Usage.InputTokens,
			CompletionTokens: resp.Usage.OutputTokens,
			TotalTokens:      resp.Usage.InputTokens + resp.Usage.OutputTokens,
		}
	}

	return json.Marshal(chatResp)
}

// ========================================================================
// OpenAI → Claude
// ========================================================================

func openAIToClaude(body []byte) ([]byte, error) {
	kind := detectBodyKind(body)
	switch kind {
	case bodyRequest:
		return openAIToClaudeRequest(body)
	case bodyResponse:
		return openAIToClaudeResponse(body)
	default:
		return nil, fmt.Errorf("cannot detect body kind for openai→claude conversion")
	}
}

func openAIToClaudeRequest(body []byte) ([]byte, error) {
	var req ChatRequest
	if err := json.Unmarshal(body, &req); err != nil {
		return nil, fmt.Errorf("openai→claude: unmarshal request: %w", err)
	}

	claude := ClaudeRequest{
		Model:     req.Model,
		MaxTokens: ptrUint(4096),
	}

	if req.MaxCompletionTokens != nil && *req.MaxCompletionTokens > 0 {
		claude.MaxTokens = ptrUint(uint(*req.MaxCompletionTokens))
	} else if req.MaxTokens != nil && *req.MaxTokens > 0 {
		claude.MaxTokens = ptrUint(uint(*req.MaxTokens))
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
	claudeMsgs := make([]ClaudeMessage, 0, len(req.Messages))

	for _, msg := range req.Messages {
		switch msg.Role {
		case RoleSystem, RoleDeveloper:
			systemParts = append(systemParts, extractTextContent(msg.Content))
			continue
		case RoleUser:
			claudeMsgs = append(claudeMsgs, ClaudeMessage{
				Role: "user", Content: convertOpenAIContentToClaude(msg.Content),
			})
		case RoleAssistant:
			cm := ClaudeMessage{Role: "assistant"}
			if msg.ReasoningContent != nil && *msg.ReasoningContent != "" {
				blocks := []ClaudeContentBlock{
					{Type: "thinking", Thinking: ptrStr(*msg.ReasoningContent)},
				}
				if content, ok := msg.Content.(string); ok && content != "" {
					blocks = append(blocks, ClaudeContentBlock{Type: "text", Text: ptrStr(content)})
				}
				if len(msg.ToolCalls) > 0 {
					for _, tc := range msg.ToolCalls {
						blocks = append(blocks, claudeToolUseFromOpenAI(tc))
					}
				}
				cm.Content = blocks
			} else if len(msg.ToolCalls) > 0 {
				blocks := make([]ClaudeContentBlock, 0, len(msg.ToolCalls)+1)
				if content, ok := msg.Content.(string); ok && content != "" {
					blocks = append(blocks, ClaudeContentBlock{Type: "text", Text: ptrStr(content)})
				}
				for _, tc := range msg.ToolCalls {
					blocks = append(blocks, claudeToolUseFromOpenAI(tc))
				}
				cm.Content = blocks
			} else {
				cm.Content = extractTextContent(msg.Content)
			}
			claudeMsgs = append(claudeMsgs, cm)
		case RoleTool:
			claudeMsgs = append(claudeMsgs, ClaudeMessage{
				Role: "user",
				Content: []ClaudeContentBlock{{
					Type: "tool_result", ToolUseID: msg.ToolCallID,
					Content: extractTextContent(msg.Content),
				}},
			})
		}
	}

	if len(systemParts) > 0 {
		claude.System = strings.Join(systemParts, "\n")
	}

	if len(req.Tools) > 0 {
		var tools []ClaudeTool
		if err := json.Unmarshal(req.Tools, &tools); err == nil {
			claude.Tools = tools
		} else {
			var otools []Tool
			if err := json.Unmarshal(req.Tools, &otools); err == nil {
				claude.Tools = make([]ClaudeTool, 0, len(otools))
				for _, ot := range otools {
					claude.Tools = append(claude.Tools, ClaudeTool{
						Name: ot.Function.Name, Description: ot.Function.Description,
						InputSchema: ot.Function.Parameters, Type: "custom",
					})
				}
			} else {
				var raw []map[string]any
				if err := json.Unmarshal(req.Tools, &raw); err == nil {
					claude.Tools = toolListOpenAIToClaude(raw)
				}
			}
		}
	}

	if req.ToolChoice != nil {
		claude.ToolChoice = mapToolChoiceOpenAIToClaude(req.ToolChoice)
	}
	if req.ReasoningEffort != "" {
		claude.Thinking = &ClaudeThinking{
			Type: "enabled", BudgetTokens: ptrInt(10000),
		}
	}

	claude.Messages = claudeMsgs
	return json.Marshal(claude)
}

func openAIToClaudeResponse(body []byte) ([]byte, error) {
	var resp ChatResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("openai→claude: unmarshal response: %w", err)
	}

	claudeResp := ClaudeResponse{
		ID: "", Type: "message", Role: "assistant", Model: resp.Model,
	}

	for _, choice := range resp.Choices {
		blocks := make([]ClaudeContentBlock, 0)
		if choice.Message.ReasoningContent != nil && *choice.Message.ReasoningContent != "" {
			blocks = append(blocks, ClaudeContentBlock{
				Type: "thinking", Thinking: ptrStr(*choice.Message.ReasoningContent),
			})
		}
		if choice.Message.Content != nil {
			blocks = append(blocks, ClaudeContentBlock{
				Type: "text", Text: choice.Message.Content,
			})
		}
		for _, tc := range choice.Message.ToolCalls {
			blocks = append(blocks, claudeToolUseFromOpenAI(tc))
		}
		claudeResp.Content = blocks
		claudeResp.StopReason = mapFinishReasonToClaude(choice.FinishReason)
	}

	if resp.Usage != nil {
		claudeResp.Usage = &ClaudeUsage{
			InputTokens: resp.Usage.PromptTokens, OutputTokens: resp.Usage.CompletionTokens,
		}
	}
	if resp.ID != "" {
		claudeResp.ID = "msg_" + resp.ID
	}
	return json.Marshal(claudeResp)
}

// ========================================================================
// Responses API ↔ OpenAI Chat
// ========================================================================

func responsesToOpenAI(body []byte) ([]byte, error) {
	kind := detectBodyKind(body)
	switch kind {
	case bodyRequest:
		return responsesToOpenAIRequest(body)
	case bodyResponse:
		return responsesToOpenAIResponse(body)
	default:
		// Try request first (most common)
		return responsesToOpenAIRequest(body)
	}
}

func responsesToOpenAIRequest(body []byte) ([]byte, error) {
	var req ResponsesRequest
	if err := json.Unmarshal(body, &req); err != nil {
		return nil, fmt.Errorf("responses→openai: unmarshal: %w", err)
	}

	chatReq := ChatRequest{Model: req.Model}
	if req.Temperature != nil {
		chatReq.Temperature = req.Temperature
	}
	if req.TopP != nil {
		chatReq.TopP = req.TopP
	}
	if req.MaxOutputTokens != nil {
		chatReq.MaxCompletionTokens = ptrInt(int(*req.MaxOutputTokens))
	}
	if req.Stream != nil {
		chatReq.Stream = req.Stream
	}

	chatReq.Messages = make([]Message, 0)

	if len(req.Instructions) > 0 {
		var instr string
		if err := json.Unmarshal(req.Instructions, &instr); err == nil && instr != "" {
			chatReq.Messages = append(chatReq.Messages, Message{Role: "system", Content: instr})
		}
	}

	if len(req.Input) > 0 {
		messages, err := parseResponsesInputToMessages(req.Input)
		if err == nil {
			chatReq.Messages = append(chatReq.Messages, messages...)
		}
	}

	chatReq.Tools = req.Tools
	chatReq.ToolChoice = mapResponseToolChoice(req.ToolChoice)

	return json.Marshal(chatReq)
}

func responsesToOpenAIResponse(body []byte) ([]byte, error) {
	// Responses response → OpenAI Chat response is rarely needed.
	// When it is, it means the upstream returned Responses format but the
	// client expects OpenAI Chat. Basic mapping only.
	return body, nil // passthrough with warning
}

func openAIToResponses(body []byte) ([]byte, error) {
	kind := detectBodyKind(body)
	switch kind {
	case bodyRequest:
		// OpenAI request → Responses request is rarely needed
		return body, nil // passthrough
	case bodyResponse:
		return openAIToResponsesResponse(body)
	default:
		return nil, fmt.Errorf("cannot detect body kind for openai→responses conversion")
	}
}

func openAIToResponsesResponse(body []byte) ([]byte, error) {
	var chatResp ChatResponse
	if err := json.Unmarshal(body, &chatResp); err != nil {
		return nil, fmt.Errorf("openai→responses: unmarshal: %w", err)
	}

	resp := ResponsesResponse{
		ID:     chatResp.ID,
		Object: "response",
		Status: "completed",
		Model:  chatResp.Model,
	}

	for _, choice := range chatResp.Choices {
		content := make([]ResponsesOutputContent, 0)
		if choice.Message.Content != nil && *choice.Message.Content != "" {
			content = append(content, ResponsesOutputContent{
				Type: "output_text", Text: *choice.Message.Content,
			})
		}
		if len(choice.Message.ToolCalls) > 0 {
			for _, tc := range choice.Message.ToolCalls {
				resp.Output = append(resp.Output, ResponsesOutput{
					Type: "function_call", ID: tc.ID, CallID: tc.ID,
					Name: tc.Function.Name, Arguments: tc.Function.Arguments,
					Status: "completed",
				})
			}
		}
		if len(content) > 0 {
			resp.Output = append(resp.Output, ResponsesOutput{
				Type: "message", ID: "msg_" + chatResp.ID,
				Status: "completed", Role: "assistant", Content: content,
			})
		}
	}

	if chatResp.Usage != nil {
		resp.Usage = &ResponsesUsage{
			InputTokens:  chatResp.Usage.PromptTokens,
			OutputTokens: chatResp.Usage.CompletionTokens,
			TotalTokens:  chatResp.Usage.TotalTokens,
		}
	}

	return json.Marshal(resp)
}

// ========================================================================
// Responses API ↔ Claude
// ========================================================================

func responsesToClaude(body []byte) ([]byte, error) {
	kind := detectBodyKind(body)
	switch kind {
	case bodyRequest:
		return responsesToClaudeRequest(body)
	case bodyResponse:
		return responsesToClaudeResponse(body)
	default:
		return responsesToClaudeRequest(body)
	}
}

func responsesToClaudeRequest(body []byte) ([]byte, error) {
	var req ResponsesRequest
	if err := json.Unmarshal(body, &req); err != nil {
		return nil, fmt.Errorf("responses→claude: unmarshal: %w", err)
	}

	claude := ClaudeRequest{Model: req.Model}
	if req.MaxOutputTokens != nil {
		claude.MaxTokens = ptrUint(*req.MaxOutputTokens)
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

	claude.Messages = make([]ClaudeMessage, 0)
	if len(req.Input) > 0 {
		msgs, err := parseResponsesInputToClaudeMessages(req.Input)
		if err == nil {
			claude.Messages = msgs
		}
	}

	return json.Marshal(claude)
}

func responsesToClaudeResponse(body []byte) ([]byte, error) {
	// Responses response → Claude response is rarely needed.
	return body, nil // passthrough
}

func claudeToResponses(body []byte) ([]byte, error) {
	kind := detectBodyKind(body)
	switch kind {
	case bodyRequest:
		return claudeToResponsesRequest(body)
	case bodyResponse:
		return claudeToResponsesResponse(body)
	default:
		return nil, fmt.Errorf("cannot detect body kind for claude→responses conversion")
	}
}

func claudeToResponsesRequest(body []byte) ([]byte, error) {
	var req ClaudeRequest
	if err := json.Unmarshal(body, &req); err != nil {
		return nil, fmt.Errorf("claude→responses: unmarshal: %w", err)
	}

	r := ResponsesRequest{Model: req.Model}
	if req.MaxTokens != nil {
		r.MaxOutputTokens = ptrUint(*req.MaxTokens)
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
		sysStr := extractTextContent(req.System)
		if sysStr != "" {
			r.Instructions = json.RawMessage(`"` + sysStr + `"`)
		}
	}

	inputItems := make([]InputItem, 0, len(req.Messages))
	for _, msg := range req.Messages {
		switch msg.Role {
		case "user":
			text := extractTextContent(msg.Content)
			if text != "" {
				inputItems = append(inputItems, InputItem{
					Type: "message", Role: "user", Content: text,
				})
			}
		case "assistant":
			text := extractTextContent(msg.Content)
			if text != "" {
				inputItems = append(inputItems, InputItem{
					Type: "message", Role: "assistant", Content: text,
				})
			}
			if blocks, ok := msg.Content.([]ClaudeContentBlock); ok {
				for _, block := range blocks {
					if block.Type == "tool_use" {
						args, _ := json.Marshal(block.Input)
						inputItems = append(inputItems, InputItem{
							Type: "message", Role: "assistant",
							Content: fmt.Sprintf(`[{"type":"output_text","text":"Tool: %s"}]`, block.Name),
						})
						inputItems = append(inputItems, InputItem{
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

func claudeToResponsesResponse(body []byte) ([]byte, error) {
	var resp ClaudeResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("claude→responses: unmarshal: %w", err)
	}

	r := ResponsesResponse{
		ID: resp.ID, Object: "response",
		Status: "completed", Model: resp.Model,
	}

	var textContent string
	var toolCalls []ResponsesOutput
	for _, block := range resp.Content {
		switch block.Type {
		case "text":
			if block.Text != nil {
				textContent += *block.Text
			}
		case "tool_use":
			args, _ := json.Marshal(block.Input)
			toolCalls = append(toolCalls, ResponsesOutput{
				Type: "function_call", ID: block.ID, CallID: block.ID,
				Name: block.Name, Arguments: string(args), Status: "completed",
			})
		}
	}

	if textContent != "" {
		r.Output = append(r.Output, ResponsesOutput{
			Type: "message", ID: "msg_" + resp.ID,
			Status: "completed", Role: "assistant",
			Content: []ResponsesOutputContent{{Type: "output_text", Text: textContent}},
		})
	}
	for _, tc := range toolCalls {
		r.Output = append(r.Output, tc)
	}

	if resp.Usage != nil {
		r.Usage = &ResponsesUsage{
			InputTokens:  resp.Usage.InputTokens,
			OutputTokens: resp.Usage.OutputTokens,
			TotalTokens:  resp.Usage.InputTokens + resp.Usage.OutputTokens,
		}
	}

	return json.Marshal(r)
}

// ========================================================================
// Mapping helpers
// ========================================================================

func mapClaudeRoleToOpenAI(role string) string {
	switch role {
	case "user", "assistant":
		return role
	default:
		return "user"
	}
}

func mapClaudeStopToOpenAI(stop *string) string {
	if stop == nil {
		return ""
	}
	switch *stop {
	case "end_turn":
		return FinishReasonStop
	case "tool_use":
		return FinishReasonToolCalls
	case "max_tokens":
		return FinishReasonLength
	case "stop_sequence":
		return FinishReasonStop
	default:
		return *stop
	}
}

func mapFinishReasonToClaude(reason string) *string {
	switch reason {
	case FinishReasonStop:
		return ptrStr("end_turn")
	case FinishReasonToolCalls:
		return ptrStr("tool_use")
	case FinishReasonLength:
		return ptrStr("max_tokens")
	default:
		return ptrStr(reason)
	}
}

// ========================================================================
// Content conversion helpers
// ========================================================================

func extractTextContent(content any) string {
	if content == nil {
		return ""
	}
	switch v := content.(type) {
	case string:
		return v
	case []ContentPart:
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

func convertOpenAIContentToClaude(content any) any {
	if content == nil {
		return ""
	}
	switch v := content.(type) {
	case string:
		if v == "" {
			return ""
		}
		return []ClaudeContentBlock{{Type: "text", Text: &v}}
	case []ContentPart:
		blocks := make([]ClaudeContentBlock, 0, len(v))
		for _, part := range v {
			switch part.Type {
			case "text":
				blocks = append(blocks, ClaudeContentBlock{Type: "text", Text: ptrStr(part.Text)})
			case "image_url":
				if part.ImageURL != nil {
					blocks = append(blocks, ClaudeContentBlock{
						Type: "image",
						Source: &ClaudeSource{Type: "url", URL: part.ImageURL.URL},
					})
				}
			case "input_audio":
				blocks = append(blocks, ClaudeContentBlock{Type: "text", Text: ptrStr("[audio input]")})
			}
		}
		return blocks
	default:
		return fmt.Sprintf("%v", v)
	}
}

func convertClaudeContentToOpenAI(content []ClaudeContentBlock) any {
	parts := make([]ContentPart, 0, len(content))
	for _, block := range content {
		switch block.Type {
		case "text":
			if block.Text != nil {
				parts = append(parts, ContentPart{Type: "text", Text: *block.Text})
			}
		case "image":
			if block.Source != nil {
				parts = append(parts, ContentPart{
					Type: "image_url",
					ImageURL: &ImageURL{URL: block.Source.URL},
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

func claudeToolUseFromOpenAI(tc ToolCall) ClaudeContentBlock {
	var input any
	if err := json.Unmarshal([]byte(tc.Function.Arguments), &input); err != nil {
		input = tc.Function.Arguments
	}
	return ClaudeContentBlock{
		Type: "tool_use", ID: tc.ID, Name: tc.Function.Name, Input: input,
	}
}

// ========================================================================
// Tool mapping helpers
// ========================================================================

func toolListOpenAIToClaude(raw []map[string]any) []ClaudeTool {
	tools := make([]ClaudeTool, 0, len(raw))
	for _, t := range raw {
		ct := ClaudeTool{Type: "custom"}
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

func mapToolChoiceOpenAIToClaude(tc any) any {
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
				return ClaudeToolChoice{Type: "tool", Name: name}
			}
		}
		return v
	case ToolChoice:
		if v.Type == "tool" && v.Name != "" {
			return ClaudeToolChoice{Type: "tool", Name: v.Name}
		}
		return map[string]any{"type": v.Type}
	default:
		return map[string]any{"type": "auto"}
	}
}

func mapToolChoiceClaudeToOpenAI(tc any) any {
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
			return ToolChoice{Type: "tool", Name: name}
		}
		return typeStr
	case ClaudeToolChoice:
		if v.Type == "tool" && v.Name != "" {
			return ToolChoice{Type: "tool", Name: v.Name}
		}
		return v.Type
	default:
		return "auto"
	}
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

// ========================================================================
// Responses input parsing
// ========================================================================

func parseResponsesInputToMessages(input json.RawMessage) ([]Message, error) {
	var str string
	if err := json.Unmarshal(input, &str); err == nil {
		return []Message{{Role: "user", Content: str}}, nil
	}

	var items []InputItem
	if err := json.Unmarshal(input, &items); err != nil {
		var raw []map[string]any
		if err := json.Unmarshal(input, &raw); err != nil {
			return nil, err
		}
		items = make([]InputItem, 0, len(raw))
		for _, r := range raw {
			item := InputItem{}
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

	msgs := make([]Message, 0, len(items))
	for _, item := range items {
		switch item.Type {
		case "input_text", "":
			msgs = append(msgs, Message{Role: "user", Content: item.Text})
		case "message":
			role := item.Role
			if role == "" {
				role = "user"
			}
			if item.Content != "" {
				msgs = append(msgs, Message{Role: role, Content: item.Content})
			}
		case "function_call_output":
			msgs = append(msgs, Message{
				Role: "tool", ToolCallID: item.CallID,
				Content: item.Output,
			})
		case "item_reference":
			continue
		}
	}
	return msgs, nil
}

func parseResponsesInputToClaudeMessages(input json.RawMessage) ([]ClaudeMessage, error) {
	var str string
	if err := json.Unmarshal(input, &str); err == nil {
		return []ClaudeMessage{{Role: "user", Content: str}}, nil
	}

	var items []InputItem
	if err := json.Unmarshal(input, &items); err != nil {
		var raw []map[string]any
		if err := json.Unmarshal(input, &raw); err != nil {
			return nil, err
		}
		items = make([]InputItem, 0, len(raw))
		for _, r := range raw {
			item := InputItem{}
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

	msgs := make([]ClaudeMessage, 0, len(items))
	for _, item := range items {
		switch item.Type {
		case "input_text", "":
			msgs = append(msgs, ClaudeMessage{Role: "user", Content: item.Text})
		case "message":
			role := item.Role
			if role == "" {
				role = "user"
			}
			if item.Content != "" {
				msgs = append(msgs, ClaudeMessage{Role: role, Content: item.Content})
			}
		case "function_call_output":
			msgs = append(msgs, ClaudeMessage{
				Role: "user",
				Content: []ClaudeContentBlock{{
					Type: "tool_result", ToolUseID: item.CallID,
					Content: item.Output,
				}},
			})
		case "item_reference":
			continue
		}
	}
	return msgs, nil
}

// ========================================================================
// Pointer helpers
// ========================================================================

func ptrStr(s string) *string  { return &s }
func ptrInt(i int) *int        { return &i }
func ptrUint(u uint) *uint     { return &u }

func safeStr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

