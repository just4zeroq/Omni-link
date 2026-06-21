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
	Register("gemini", &GeminiExecutor{})
}

// GeminiExecutor handles Google Gemini API endpoints.
// Native format: gemini.
type GeminiExecutor struct {
	channel any
}

func (e *GeminiExecutor) Init(channel any) {
	e.channel = channel
}

func (e *GeminiExecutor) GetName() string {
	if ch, ok := e.channel.(interface{ GetName() string }); ok {
		return ch.GetName()
	}
	return "Gemini"
}

func (e *GeminiExecutor) NativeFormats() []EndpointCapability {
	return []EndpointCapability{
		{Format: translator.FormatGemini, RelayMode: translator.RelayModeChatCompletions},
	}
}

func (e *GeminiExecutor) GetRequestURL(info *RequestInfo) (string, error) {
	baseURL := info.BaseURL
	if baseURL == "" {
		baseURL = "https://generativelanguage.googleapis.com"
	}
	baseURL = strings.TrimSuffix(baseURL, "/")
	model := info.ActualModelName
	if model == "" {
		model = info.Model
	}

	if info.IsStream {
		return fmt.Sprintf("%s/v1beta/models/%s:streamGenerateContent?alt=sse", baseURL, model), nil
	}
	return fmt.Sprintf("%s/v1beta/models/%s:generateContent", baseURL, model), nil
}

func (e *GeminiExecutor) SetupRequestHeader(header http.Header, info *RequestInfo) error {
	header.Set("x-goog-api-key", info.ApiKey)
	header.Set("Content-Type", "application/json")
	if info.IsStream {
		header.Set("Accept", "text/event-stream")
	} else {
		header.Set("Accept", "application/json")
	}
	return nil
}

func (e *GeminiExecutor) ConvertRequest(body []byte, from, to translator.Format) ([]byte, error) {
	if from == to {
		return body, nil
	}

	// Hub: any non-openai → openai first
	if from != translator.FormatOpenAI {
		var err error
		body, err = translator.Convert(body, from, translator.FormatOpenAI)
		if err != nil {
			return nil, fmt.Errorf("gemini: conv %s→openai: %w", from, err)
		}
	}

	return openAIToGeminiRequest(body)
}

func (e *GeminiExecutor) ConvertResponse(body []byte, from, to translator.Format) ([]byte, error) {
	if from == to {
		return body, nil
	}
	if from != translator.FormatGemini {
		return nil, fmt.Errorf("gemini: unsupported response conv %s→%s", from, to)
	}

	switch to {
	case translator.FormatOpenAI:
		return geminiToOpenAIResponse(body)
	case translator.FormatClaude:
		// Gemini→Claude: via OpenAI intermediate
		oaBody, err := geminiToOpenAIResponse(body)
		if err != nil {
			return nil, err
		}
		return translator.Convert(oaBody, translator.FormatOpenAI, to)
	default:
		return nil, fmt.Errorf("gemini: unsupported response conv gemini→%s", to)
	}
}

func (e *GeminiExecutor) RequestCustomize(body []byte, info *RequestInfo) []byte {
	return body
}

func (e *GeminiExecutor) ResponseCustomize(body []byte, info *RequestInfo) []byte {
	return body
}

func (e *GeminiExecutor) NewResponseStream(from, to translator.Format) (ResponseStream, error) {
	if from == to {
		return nil, nil
	}
	return nil, nil // streaming conversion not yet implemented for gemini
}

func (e *GeminiExecutor) DoRequest(info *RequestInfo, body io.Reader) (*http.Response, error) {
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

	client := &http.Client{Timeout: 120 * time.Second}
	return client.Do(httpReq)
}

// ========================================================================
// OpenAI → Gemini Request converter
// ========================================================================

func openAIToGeminiRequest(body []byte) ([]byte, error) {
	// Parse as generic OpenAI request using translator types
	// Then rebuild as Gemini request
	var req translator.ChatRequest
	if err := json.Unmarshal(body, &req); err != nil {
		return nil, fmt.Errorf("openai→gemini: unmarshal: %w", err)
	}

	gReq := translator.GeminiChatRequest{
		Contents: make([]translator.GeminiContent, 0),
		GenerationConfig: &translator.GeminiGenerationConfig{
			Temperature: req.Temperature,
			TopP:        req.TopP,
		},
	}

	if req.MaxCompletionTokens != nil && *req.MaxCompletionTokens > 0 {
		gReq.GenerationConfig.MaxOutputTokens = uintPtr(uint(*req.MaxCompletionTokens))
	} else if req.MaxTokens != nil && *req.MaxTokens > 0 {
		gReq.GenerationConfig.MaxOutputTokens = uintPtr(uint(*req.MaxTokens))
	}

	if req.Stop != nil {
		gReq.GenerationConfig.StopSequences = parseStopSequences(req.Stop)
	}
	if req.Seed != nil {
		gReq.GenerationConfig.Seed = int64Ptr(*req.Seed)
	}

	// Gather system instructions and convert messages
	var sysParts []translator.GeminiPart
	for _, msg := range req.Messages {
		switch msg.Role {
		case "system", "developer":
			text := geminiExtractText(msg.Content)
			if text != "" {
				sysParts = append(sysParts, translator.GeminiPart{Text: text})
			}
		case "user":
			parts := convertUserPartsToGemini(msg.Content)
			if len(parts) > 0 {
				gReq.Contents = append(gReq.Contents, translator.GeminiContent{
					Role: "user", Parts: parts,
				})
			}
		case "assistant":
			parts := convertAssistantPartsToGemini(msg)
			if len(parts) > 0 {
				gReq.Contents = append(gReq.Contents, translator.GeminiContent{
					Role: "model", Parts: parts,
				})
			}
		case "tool":
			text := geminiExtractText(msg.Content)
			var response any = text
			if text != "" {
				var parsed any
				if json.Unmarshal([]byte(text), &parsed) == nil {
					response = parsed
				}
			}
			// Tool results appended to last user content
			if len(gReq.Contents) == 0 || gReq.Contents[len(gReq.Contents)-1].Role == "model" {
				gReq.Contents = append(gReq.Contents, translator.GeminiContent{Role: "user"})
			}
			last := len(gReq.Contents) - 1
			gReq.Contents[last].Parts = append(gReq.Contents[last].Parts, translator.GeminiPart{
				FunctionResponse: &translator.GeminiFunctionResponse{
					Name: msg.ToolCallID, Response: response,
				},
			})
		}
	}

	if len(sysParts) > 0 {
		gReq.SystemInstruction = &translator.GeminiContent{Parts: sysParts}
	}

	// Tools
	if len(req.Tools) > 0 {
		gReq.Tools = req.Tools // pass through as raw JSON
	}
	if req.ToolChoice != nil {
		gReq.ToolConfig = mapToolChoiceToGemini(req.ToolChoice)
	}

	// Safety settings
	gReq.SafetySettings = []translator.GeminiSafetySetting{
		{Category: "HARM_CATEGORY_HARASSMENT", Threshold: "BLOCK_ONLY_HIGH"},
		{Category: "HARM_CATEGORY_HATE_SPEECH", Threshold: "BLOCK_ONLY_HIGH"},
		{Category: "HARM_CATEGORY_SEXUALLY_EXPLICIT", Threshold: "BLOCK_ONLY_HIGH"},
		{Category: "HARM_CATEGORY_DANGEROUS_CONTENT", Threshold: "BLOCK_ONLY_HIGH"},
	}

	// Reasoning effort → thinking config
	if req.ReasoningEffort != "" {
		gReq.GenerationConfig.ThinkingConfig = mapReasoningEffortToGemini(req.ReasoningEffort)
	}

	return json.Marshal(gReq)
}

// ========================================================================
// Gemini → OpenAI Response converter
// ========================================================================

func geminiToOpenAIResponse(body []byte) ([]byte, error) {
	var geminiResp translator.GeminiChatResponse
	if err := json.Unmarshal(body, &geminiResp); err != nil {
		return nil, fmt.Errorf("gemini→openai: unmarshal: %w", err)
	}

	resp := translator.ChatResponse{
		ID:      fmt.Sprintf("chatcmpl-%d", time.Now().UnixNano()),
		Object:  "chat.completion",
		Created: time.Now().Unix(),
		Choices: make([]translator.Choice, 0),
	}

	if len(geminiResp.Candidates) == 0 {
		resp.Model = "gemini"
		result, _ := json.Marshal(resp)
		return result, nil
	}

	candidate := geminiResp.Candidates[0]
	model := geminiResp.ModelName
	if model == "" {
		model = "gemini"
	}
	resp.Model = model

	choice := translator.Choice{
		Index:        candidate.Index,
		FinishReason: mapGeminiFinishToOpenAI(candidate.FinishReason),
	}

	if candidate.Content != nil {
		var textParts []string
		var reasoningParts []string
		var toolCalls []translator.ToolCall
		idx := 0

		for _, part := range candidate.Content.Parts {
			isThought := part.Thought != nil && *part.Thought
			if part.Text != "" {
				if isThought {
					reasoningParts = append(reasoningParts, part.Text)
				} else {
					textParts = append(textParts, part.Text)
				}
			}
			if part.InlineData != nil {
				textParts = append(textParts, fmt.Sprintf("![image](data:%s;base64,%s)", part.InlineData.MimeType, part.InlineData.Data))
			}
			if part.ExecutableCode != nil {
				textParts = append(textParts, fmt.Sprintf("```%s\n%s\n```", part.ExecutableCode.Language, part.ExecutableCode.Code))
			}
			if part.CodeExecutionResult != nil {
				textParts = append(textParts, fmt.Sprintf("Execution %s:\n%s", part.CodeExecutionResult.Outcome, part.CodeExecutionResult.Output))
			}
			if part.FunctionCall != nil {
				argsJSON, _ := json.Marshal(part.FunctionCall.Arguments)
				toolCalls = append(toolCalls, translator.ToolCall{
					ID:   fmt.Sprintf("call_%d", idx),
					Type: "function",
					Function: translator.FunctionCall{
						Name:      part.FunctionCall.FunctionName,
						Arguments: string(argsJSON),
					},
				})
				idx++
			}
		}

		msg := translator.ChatMessage{Role: "assistant"}
		msg.Content = strPtr(strings.Join(textParts, ""))
		if len(reasoningParts) > 0 {
			joined := strings.Join(reasoningParts, "\n")
			msg.ReasoningContent = &joined
		}
		if len(toolCalls) > 0 {
			msg.ToolCalls = toolCalls
			choice.FinishReason = "tool_calls"
		}
		choice.Message = msg
	}

	resp.Choices = append(resp.Choices, choice)

	if geminiResp.UsageMetadata != nil {
		resp.Usage = &translator.ChatUsage{
			PromptTokens:     geminiResp.UsageMetadata.PromptTokenCount,
			CompletionTokens: geminiResp.UsageMetadata.CandidatesTokenCount,
			TotalTokens:      geminiResp.UsageMetadata.TotalTokenCount,
		}
	}

	return json.Marshal(resp)
}

// ========================================================================
// Mapping helpers
// ========================================================================

func convertUserPartsToGemini(content any) []translator.GeminiPart {
	switch v := content.(type) {
	case string:
		if v == "" {
			return nil
		}
		return []translator.GeminiPart{{Text: v}}
	case []translator.ContentPart:
		var parts []translator.GeminiPart
		for _, part := range v {
			switch part.Type {
			case "text":
				if part.Text != "" {
					parts = append(parts, translator.GeminiPart{Text: part.Text})
				}
			case "image_url":
				if part.ImageURL != nil {
					if mimeType, data, ok := parseDataURL(part.ImageURL.URL); ok {
						parts = append(parts, translator.GeminiPart{
							InlineData: &translator.GeminiInlineData{MimeType: mimeType, Data: data},
						})
					}
				}
			case "input_audio":
				if part.InputAudio != nil {
					mimeType := "audio/wav"
					if part.InputAudio.Format != "" {
						mimeType = "audio/" + part.InputAudio.Format
					}
					parts = append(parts, translator.GeminiPart{
						InlineData: &translator.GeminiInlineData{MimeType: mimeType, Data: part.InputAudio.Data},
					})
				}
			}
		}
		return parts
	default:
		return nil
	}
}

func convertAssistantPartsToGemini(msg translator.Message) []translator.GeminiPart {
	var parts []translator.GeminiPart
	text := geminiExtractText(msg.Content)
	if text != "" {
		parts = append(parts, translator.GeminiPart{Text: text})
	}
	if msg.ReasoningContent != nil && *msg.ReasoningContent != "" {
		t := true
		parts = append(parts, translator.GeminiPart{
			Text: *msg.ReasoningContent, Thought: &t,
		})
	}
	for _, tc := range msg.ToolCalls {
		args := map[string]any{}
		if tc.Function.Arguments != "" {
			json.Unmarshal([]byte(tc.Function.Arguments), &args)
		}
		parts = append(parts, translator.GeminiPart{
			FunctionCall: &translator.GeminiFunctionCall{
				FunctionName: tc.Function.Name, Arguments: args,
			},
		})
	}
	return parts
}

func mapToolChoiceToGemini(tc any) any {
	if tc == nil {
		return nil
	}
	switch v := tc.(type) {
	case string:
		switch v {
		case "auto":
			return map[string]any{"functionCallingConfig": map[string]any{"mode": "AUTO"}}
		case "none":
			return map[string]any{"functionCallingConfig": map[string]any{"mode": "NONE"}}
		case "required":
			return map[string]any{"functionCallingConfig": map[string]any{"mode": "ANY"}}
		default:
			return map[string]any{"functionCallingConfig": map[string]any{"mode": "AUTO"}}
		}
	case map[string]any:
		if v["type"] == "function" {
			cfg := map[string]any{"functionCallingConfig": map[string]any{"mode": "ANY"}}
			if fn, ok := v["function"].(map[string]any); ok {
				if name, ok := fn["name"].(string); ok && name != "" {
					cfg["functionCallingConfig"].(map[string]any)["allowedFunctionNames"] = []string{name}
				}
			}
			return cfg
		}
	}
	return nil
}

func mapReasoningEffortToGemini(effort string) *translator.GeminiThinkingConfig {
	var budget int
	var level string
	switch effort {
	case "low":
		budget = 1024
		level = "LOW"
	case "medium":
		budget = 8192
		level = "MEDIUM"
	case "high":
		budget = 32768
		level = "HIGH"
	default:
		budget = 8192
		level = "MEDIUM"
	}
	return &translator.GeminiThinkingConfig{
		IncludeThoughts: true,
		ThoughtBudget:   &budget,
		ThinkingLevel:   level,
	}
}

func mapGeminiFinishToOpenAI(reason string) string {
	switch reason {
	case "STOP":
		return "stop"
	case "MAX_TOKENS":
		return "length"
	case "SAFETY", "BLOCKLIST", "PROHIBITED_CONTENT", "SPAM":
		return "content_filter"
	case "RECITATION":
		return "content_filter"
	case "OTHER":
		return "stop"
	case "TOOL_CALLS", "FUNCTION_CALL":
		return "tool_calls"
	default:
		return reason
	}
}

func parseStopSequences(stop any) []string {
	if stop == nil {
		return nil
	}
	switch v := stop.(type) {
	case string:
		if v != "" {
			return []string{v}
		}
	case []string:
		return v
	case []any:
		var result []string
		for _, item := range v {
			if s, ok := item.(string); ok && s != "" {
				result = append(result, s)
			}
		}
		return result
	}
	return nil
}

func parseDataURL(dataURL string) (mimeType, data string, ok bool) {
	if len(dataURL) < 11 || dataURL[:5] != "data:" {
		return "", "", false
	}
	rest := dataURL[5:]
	semiIdx := strings.Index(rest, ";")
	if semiIdx == -1 {
		return "", "", false
	}
	mimeType = rest[:semiIdx]
	afterSemi := rest[semiIdx+1:]
	if len(afterSemi) < 7 || afterSemi[:7] != "base64," {
		return "", "", false
	}
	return mimeType, afterSemi[7:], true
}

func geminiExtractText(content any) string {
	if content == nil {
		return ""
	}
	switch v := content.(type) {
	case string:
		return v
	default:
		b, err := json.Marshal(v)
		if err != nil {
			return fmt.Sprintf("%v", v)
		}
		return string(b)
	}
}

func uintPtr(u uint) *uint   { return &u }
func int64Ptr(i int64) *int64 { return &i }
func strPtr(s string) *string { return &s }
