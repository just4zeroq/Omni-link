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
	Register("volcengine", &VolcengineExecutor{})
}

// VolcengineExecutor handles 火山引擎 (ByteDance/Doubao) API endpoints.
// OpenAI-compatible with volcengine-specific URL routing.
type VolcengineExecutor struct {
	channel any
}

func (e *VolcengineExecutor) Init(channel any) {
	e.channel = channel
}

func (e *VolcengineExecutor) GetName() string {
	if ch, ok := e.channel.(interface{ GetName() string }); ok {
		return ch.GetName()
	}
	return "Volcengine"
}

func (e *VolcengineExecutor) NativeFormats() []EndpointCapability {
	return []EndpointCapability{
		{Format: translator.FormatOpenAI, RelayMode: translator.RelayModeChatCompletions},
	}
}

func (e *VolcengineExecutor) GetRequestURL(info *RequestInfo) (string, error) {
	baseURL := info.BaseURL
	if baseURL == "" {
		baseURL = "https://ark.cn-beijing.volces.com"
	}
	baseURL = strings.TrimSuffix(baseURL, "/")

	model := info.ActualModelName
	if model == "" {
		model = info.Model
	}

	switch info.RelayMode {
	case translator.RelayModeChatCompletions:
		if strings.HasPrefix(model, "bot-") {
			return baseURL + "/api/v3/bots/chat/completions", nil
		}
		return baseURL + "/api/v3/chat/completions", nil
	case translator.RelayModeClaudeMessages:
		return baseURL + "/api/coding/v1/messages", nil
	default:
		return baseURL + "/api/v3/chat/completions", nil
	}
}

func (e *VolcengineExecutor) SetupRequestHeader(header http.Header, info *RequestInfo) error {
	header.Set("Authorization", "Bearer "+info.ApiKey)
	header.Set("Content-Type", "application/json")
	if info.IsStream {
		header.Set("Accept", "text/event-stream")
	} else {
		header.Set("Accept", "application/json")
	}
	return nil
}

func (e *VolcengineExecutor) ConvertRequest(body []byte, from, to translator.Format) ([]byte, error) {
	if from == to {
		return body, nil
	}

	// OpenAI-compatible endpoint — convert any non-openai to openai
	if to == translator.FormatOpenAI && from != translator.FormatOpenAI {
		return translator.Convert(body, from, translator.FormatOpenAI)
	}

	return nil, fmt.Errorf("volcengine: unsupported request conv %s→%s", from, to)
}

func (e *VolcengineExecutor) ConvertResponse(body []byte, from, to translator.Format) ([]byte, error) {
	if from == to {
		return body, nil
	}
	return nil, fmt.Errorf("volcengine: unsupported response conv %s→%s", from, to)
}

func (e *VolcengineExecutor) RequestCustomize(body []byte, info *RequestInfo) []byte {
	// Model mapping
	if info.ActualModelName != "" {
		var raw map[string]json.RawMessage
		if err := json.Unmarshal(body, &raw); err == nil {
			raw["model"], _ = json.Marshal(info.ActualModelName)
			body, _ = json.Marshal(raw)
		}
	}
	// Stream options
	if info.IsStream {
		var raw map[string]json.RawMessage
		if err := json.Unmarshal(body, &raw); err == nil {
			if _, exists := raw["stream_options"]; !exists {
				raw["stream_options"] = json.RawMessage(`{"include_usage":true}`)
				body, _ = json.Marshal(raw)
			}
		}
	}
	return body
}

func (e *VolcengineExecutor) ResponseCustomize(body []byte, info *RequestInfo) []byte {
	return body
}

func (e *VolcengineExecutor) NewResponseStream(from, to translator.Format) (ResponseStream, error) {
	if from == to {
		return nil, nil
	}
	return nil, nil // passthrough — volcengine responses are already OpenAI-compatible
}

func (e *VolcengineExecutor) DoRequest(info *RequestInfo, body io.Reader) (*http.Response, error) {
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
