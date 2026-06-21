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
	return translator.Convert(body, from, to)
}

func (e *OpenAIExecutor) ConvertResponse(body []byte, from, to translator.Format) ([]byte, error) {
	if from == to {
		return body, nil
	}
	return translator.Convert(body, from, to)
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
// Shared vendor helpers (used by multiple executors)
// ========================================================================

// replaceModelField replaces the "model" field in a JSON request body.
func replaceModelField(body []byte, model string) []byte {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(body, &raw); err != nil {
		return body
	}
	raw["model"], _ = json.Marshal(model)
	result, _ := json.Marshal(raw)
	return result
}

// injectStreamOptionsOpenAI injects stream_options.include_usage into an OpenAI request.
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
