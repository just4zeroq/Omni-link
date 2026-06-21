package translator

import "encoding/json"

// ========== OpenAI Chat Completions 请求 ==========

// ChatRequest OpenAI /v1/chat/completions 请求
// 参考 OpenAI协议文档 §3.1 完整参数表
type ChatRequest struct {
	Model                string          `json:"model"`
	Messages             []Message       `json:"messages"`
	MaxCompletionTokens  *int            `json:"max_completion_tokens,omitempty"`
	Temperature          *float64        `json:"temperature,omitempty"`
	TopP                 *float64        `json:"top_p,omitempty"`
	N                    *int            `json:"n,omitempty"`
	Stream               *bool           `json:"stream,omitempty"`
	StreamOptions        *StreamOptions  `json:"stream_options,omitempty"`
	Stop                 any             `json:"stop,omitempty"`               // string | []string
	PresencePenalty      *float64        `json:"presence_penalty,omitempty"`
	FrequencyPenalty     *float64        `json:"frequency_penalty,omitempty"`
	LogitBias            map[string]int  `json:"logit_bias,omitempty"`
	Logprobs             *bool           `json:"logprobs,omitempty"`
	TopLogprobs          *int            `json:"top_logprobs,omitempty"`
	ResponseFormat       *ResponseFormat `json:"response_format,omitempty"`
	Seed                 *int64          `json:"seed,omitempty"`
	ServiceTier          string          `json:"service_tier,omitempty"`
	Tools                json.RawMessage `json:"tools,omitempty"`          // []Tool — raw for flexibility
	ToolChoice           any             `json:"tool_choice,omitempty"`    // string | ToolChoice
	ParallelToolCalls    *bool           `json:"parallel_tool_calls,omitempty"`
	ReasoningEffort      string          `json:"reasoning_effort,omitempty"`
	User                 string          `json:"user,omitempty"`
	PromptCacheKey       string          `json:"prompt_cache_key,omitempty"`
	PromptCacheRetention string          `json:"prompt_cache_retention,omitempty"`
	Metadata             map[string]any  `json:"metadata,omitempty"`
	Modalities           []string        `json:"modalities,omitempty"`
	Audio                json.RawMessage `json:"audio,omitempty"`
	Store                *bool           `json:"store,omitempty"`
	WebSearchOptions     json.RawMessage `json:"web_search_options,omitempty"`
	SafetyIdentifier     string          `json:"safety_identifier,omitempty"`
	Verbosity            string          `json:"verbosity,omitempty"`
	Prediction           json.RawMessage `json:"prediction,omitempty"`
	MaxTokens            *int            `json:"max_tokens,omitempty"`    // deprecated
}

// StreamOptions 流式选项
type StreamOptions struct {
	IncludeUsage       bool `json:"include_usage"`
	IncludeObfuscation bool `json:"include_obfuscation,omitempty"`
}

// ResponseFormat 输出格式控制
type ResponseFormat struct {
	Type            string          `json:"type"`       // "text" | "json_object" | "json_schema"
	JSONSchema      json.RawMessage `json:"json_schema,omitempty"`
}

// ========== Message 定义 ==========

// Message OpenAI 消息格式
// content: string（纯文本）或 []ContentPart（多模态）
type Message struct {
	Role             string          `json:"role"`
	Content          any             `json:"content"`                      // string | []ContentPart
	Refusal          *string         `json:"refusal,omitempty"`            // structured output refusal
	ReasoningContent *string         `json:"reasoning_content,omitempty"`  // reasoning model thinking
	Name             string          `json:"name,omitempty"`
	ToolCalls        []ToolCall      `json:"tool_calls,omitempty"`
	ToolCallID       string          `json:"tool_call_id,omitempty"`
	Annotations      []any           `json:"annotations,omitempty"`
}

// ContentPart 多模态内容块
type ContentPart struct {
	Type       string      `json:"type"`                  // "text" | "image_url" | "input_audio" | "file"
	Text       string      `json:"text,omitempty"`
	ImageURL   *ImageURL   `json:"image_url,omitempty"`
	InputAudio *InputAudio `json:"input_audio,omitempty"`
	File       *FileData   `json:"file,omitempty"`
}

// ImageURL 图片 URL
type ImageURL struct {
	URL    string `json:"url"`
	Detail string `json:"detail,omitempty"`
}

// InputAudio 音频输入
type InputAudio struct {
	Data   string `json:"data"`
	Format string `json:"format"` // "wav" | "mp3"
}

// FileData 文件附件
type FileData struct {
	FileID   string `json:"file_id,omitempty"`
	FileData string `json:"file_data,omitempty"`
	Filename string `json:"filename,omitempty"`
}

// ========== Tool 定义 ==========

// Tool 工具定义
type Tool struct {
	Type     string      `json:"type"`     // "function"
	Function FunctionDef `json:"function"`
}

// FunctionDef 函数定义
type FunctionDef struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	Parameters  any             `json:"parameters,omitempty"`
	Strict      *bool           `json:"strict,omitempty"`
}

// ToolCall 工具调用
type ToolCall struct {
	Index    int          `json:"index,omitempty"`
	ID       string       `json:"id"`
	Type     string       `json:"type"`
	Function FunctionCall `json:"function"`
}

// FunctionCall 函数调用
type FunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"` // JSON string
}

// ToolChoice 工具选择控制
type ToolChoice struct {
	Type                   string `json:"type"`                              // "auto" | "any" | "tool" | "none"
	Name                   string `json:"name,omitempty"`                    // tool name when type="tool"
	DisableParallelToolUse bool   `json:"disable_parallel_tool_use,omitempty"`
}

// ========== 非流式响应 ==========

// ChatResponse OpenAI /v1/chat/completions 非流式响应
type ChatResponse struct {
	ID                string           `json:"id"`
	Object            string           `json:"object"`            // "chat.completion"
	Created           int64            `json:"created"`
	Model             string           `json:"model"`
	Choices           []Choice         `json:"choices"`
	Usage             *ChatUsage       `json:"usage,omitempty"`
	ServiceTier       string           `json:"service_tier,omitempty"`
	SystemFingerprint string           `json:"system_fingerprint,omitempty"` // deprecated
}

// Choice 响应选择项
type Choice struct {
	Index        int             `json:"index"`
	Message      ChatMessage     `json:"message"`
	Logprobs     any             `json:"logprobs,omitempty"`
	FinishReason string          `json:"finish_reason"`
}

// ChatMessage 响应中的消息
type ChatMessage struct {
	Role             string          `json:"role"`
	Content          *string         `json:"content"`           // null when tool_calls
	Refusal          *string         `json:"refusal,omitempty"`
	ReasoningContent *string         `json:"reasoning_content,omitempty"`
	ToolCalls        []ToolCall      `json:"tool_calls,omitempty"`
	Annotations      []any           `json:"annotations,omitempty"`
}

// ChatUsage OpenAI 用量
type ChatUsage struct {
	PromptTokens              int                        `json:"prompt_tokens"`
	CompletionTokens          int                        `json:"completion_tokens"`
	TotalTokens               int                        `json:"total_tokens"`
	PromptTokensDetails       *PromptTokensDetails        `json:"prompt_tokens_details,omitempty"`
	CompletionTokensDetails   *CompletionTokensDetails    `json:"completion_tokens_details,omitempty"`
}

// PromptTokensDetails 输入 token 细分
type PromptTokensDetails struct {
	CachedTokens int `json:"cached_tokens,omitempty"`
	AudioTokens  int `json:"audio_tokens,omitempty"`
}

// CompletionTokensDetails 输出 token 细分
type CompletionTokensDetails struct {
	ReasoningTokens          int `json:"reasoning_tokens,omitempty"`
	AudioTokens              int `json:"audio_tokens,omitempty"`
	AcceptedPredictionTokens int `json:"accepted_prediction_tokens,omitempty"`
	RejectedPredictionTokens int `json:"rejected_prediction_tokens,omitempty"`
}

// ========== 流式响应 ==========

// ChatStreamChunk OpenAI SSE 流式 chunk
type ChatStreamChunk struct {
	ID                string           `json:"id"`
	Object            string           `json:"object"`            // "chat.completion.chunk"
	Created           int64            `json:"created"`
	Model             string           `json:"model"`
	Choices           []StreamChoice   `json:"choices"`
	Usage             *ChatUsage       `json:"usage,omitempty"`   // only in final chunk with include_usage
	SystemFingerprint string           `json:"system_fingerprint,omitempty"`
}

// StreamChoice 流式选择项
type StreamChoice struct {
	Index        int              `json:"index"`
	Delta        StreamDelta      `json:"delta"`
	Logprobs     any              `json:"logprobs,omitempty"`
	FinishReason *string          `json:"finish_reason"` // null until final chunk
}

// StreamDelta 流式增量内容
type StreamDelta struct {
	Role             string          `json:"role,omitempty"`              // only first chunk
	Content          string          `json:"content,omitempty"`
	ReasoningContent *string         `json:"reasoning_content,omitempty"`
	ToolCalls        []ToolCall      `json:"tool_calls,omitempty"`       // incremental
}

// SSEEvent represents a raw SSE line with event type and data.
type SSEEvent struct {
	Event string
	Data  []byte
}
