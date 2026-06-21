package translator

import "encoding/json"

// ========== Claude Messages API 请求 ==========

// ClaudeRequest Claude /v1/messages 请求
// 参考 Claude协议文档 §3 请求体参数
type ClaudeRequest struct {
	Model         string           `json:"model"`
	Messages      []ClaudeMessage  `json:"messages"`
	MaxTokens     *uint            `json:"max_tokens,omitempty"`      // Claude 必填
	System        any              `json:"system,omitempty"`          // string | []ClaudeContentBlock
	Temperature   *float64         `json:"temperature,omitempty"`
	TopP          *float64         `json:"top_p,omitempty"`
	TopK          *int             `json:"top_k,omitempty"`
	StopSequences []string         `json:"stop_sequences,omitempty"`
	Stream        *bool            `json:"stream,omitempty"`
	Tools         []ClaudeTool     `json:"tools,omitempty"`
	ToolChoice    any              `json:"tool_choice,omitempty"`     // string | ClaudeToolChoice
	Thinking      *ClaudeThinking  `json:"thinking,omitempty"`
	Metadata      any              `json:"metadata,omitempty"`
	ServiceTier   string           `json:"service_tier,omitempty"`    // "auto" | "standard_only"
	Container     any              `json:"container,omitempty"`
	McpServers    json.RawMessage  `json:"mcp_servers,omitempty"`
}

// ClaudeMessage Claude 消息
type ClaudeMessage struct {
	Role    string `json:"role"`    // "user" | "assistant"
	Content any    `json:"content"` // string | []ClaudeContentBlock
}

// ClaudeContentBlock Claude 内容块（联合体 — 所有类型字段合并）
type ClaudeContentBlock struct {
	Type         string              `json:"type"`
	Text         *string             `json:"text,omitempty"`
	Thinking     *string             `json:"thinking,omitempty"`
	Signature    string              `json:"signature,omitempty"`
	ID           string              `json:"id,omitempty"`
	Name         string              `json:"name,omitempty"`
	Input        any                 `json:"input,omitempty"`
	ToolUseID    string              `json:"tool_use_id,omitempty"`
	Content      any                 `json:"content,omitempty"`
	IsError      *bool               `json:"is_error,omitempty"`
	Source       *ClaudeSource       `json:"source,omitempty"`
	CacheControl *ClaudeCacheControl `json:"cache_control,omitempty"`
	ServerName   string              `json:"server_name,omitempty"`
}

// ClaudeSource 多模态内容源
type ClaudeSource struct {
	Type      string `json:"type"`                // "base64" | "url"
	MediaType string `json:"media_type,omitempty"` // "image/jpeg" | "image/png" | ...
	Data      string `json:"data,omitempty"`
	URL       string `json:"url,omitempty"`
}

// ClaudeCacheControl 缓存控制
type ClaudeCacheControl struct {
	Type string `json:"type"` // "ephemeral"
}

// ClaudeThinking 扩展思考配置
type ClaudeThinking struct {
	Type         string `json:"type"`          // "enabled" | "disabled"
	BudgetTokens *int   `json:"budget_tokens,omitempty"`
}

// ClaudeTool Claude 工具定义
type ClaudeTool struct {
	Name         string              `json:"name"`
	Description  string              `json:"description,omitempty"`
	InputSchema  any                 `json:"input_schema,omitempty"`
	Type         string              `json:"type,omitempty"`
	CacheControl *ClaudeCacheControl `json:"cache_control,omitempty"`
}

// ClaudeToolChoice Claude 工具选择
type ClaudeToolChoice struct {
	Type                   string `json:"type"`                              // "auto" | "any" | "tool" | "none"
	Name                   string `json:"name,omitempty"`
	DisableParallelToolUse bool   `json:"disable_parallel_tool_use,omitempty"`
}

// ========== 非流式响应 ==========

// ClaudeResponse Claude /v1/messages 响应（流式和非流式共用结构）
type ClaudeResponse struct {
	ID           string               `json:"id,omitempty"`
	Type         string               `json:"type"`         // "message" | 流式事件类型
	Role         string               `json:"role,omitempty"`
	Content      []ClaudeContentBlock `json:"content,omitempty"`
	Model        string               `json:"model,omitempty"`
	StopReason   *string              `json:"stop_reason,omitempty"`
	StopSequence *string              `json:"stop_sequence,omitempty"`
	Usage        *ClaudeUsage         `json:"usage,omitempty"`
	Container    any                  `json:"container,omitempty"`

	// 流式事件字段
	Index        *int                `json:"index,omitempty"`
	ContentBlock *ClaudeContentBlock `json:"content_block,omitempty"`
	Delta        *ClaudeDelta        `json:"delta,omitempty"`
	Message      *ClaudeMessageInfo  `json:"message,omitempty"`
	Error        any                 `json:"error,omitempty"`
}

// ClaudeDelta 流式增量
type ClaudeDelta struct {
	Type         string  `json:"type,omitempty"`    // "text_delta" | "input_json_delta" | "thinking_delta" | "signature_delta"
	Text         *string `json:"text,omitempty"`
	PartialJSON  *string `json:"partial_json,omitempty"`
	Thinking     *string `json:"thinking,omitempty"`
	Signature    string  `json:"signature,omitempty"`
	StopReason   *string `json:"stop_reason,omitempty"`
	StopSequence *string `json:"stop_sequence,omitempty"`
}

// ClaudeMessageInfo message_start 事件中的 message 对象
type ClaudeMessageInfo struct {
	ID           string               `json:"id"`
	Type         string               `json:"type"`
	Role         string               `json:"role"`
	Content      []ClaudeContentBlock `json:"content"`
	Model        string               `json:"model"`
	StopReason   *string              `json:"stop_reason,omitempty"`
	StopSequence *string              `json:"stop_sequence,omitempty"`
	Usage        *ClaudeUsage         `json:"usage,omitempty"`
}

// ClaudeUsage Claude 用量
type ClaudeUsage struct {
	InputTokens              int              `json:"input_tokens"`
	OutputTokens             int              `json:"output_tokens"`
	CacheCreationInputTokens int              `json:"cache_creation_input_tokens,omitempty"`
	CacheReadInputTokens     int              `json:"cache_read_input_tokens,omitempty"`
	CacheCreation            *ClaudeCacheGen  `json:"cache_creation,omitempty"`
	ServerToolUse            *ClaudeServerTool `json:"server_tool_use,omitempty"`
	ServiceTier              string           `json:"service_tier,omitempty"`
}

// ClaudeCacheGen 缓存创建按 TTL 细分
type ClaudeCacheGen struct {
	Ephemeral5mInputTokens int `json:"ephemeral_5m_input_tokens,omitempty"`
	Ephemeral1hInputTokens int `json:"ephemeral_1h_input_tokens,omitempty"`
}

// ClaudeServerTool 内置工具使用
type ClaudeServerTool struct {
	WebSearchRequests int `json:"web_search_requests,omitempty"`
}

// ========== SSE 流式事件类型常量 ==========

const (
	ClaudeSSEMessageStart     = "message_start"
	ClaudeSSEContentBlockStart = "content_block_start"
	ClaudeSSEContentBlockDelta = "content_block_delta"
	ClaudeSSEContentBlockStop  = "content_block_stop"
	ClaudeSSEMessageDelta     = "message_delta"
	ClaudeSSEMessageStop      = "message_stop"
	ClaudeSSEPing             = "ping"
	ClaudeSSEError            = "error"
)

// SSE event types for the Responses API streaming.
const (
	ResponsesSSEResponseCreated          = "response.created"
	ResponsesSSEResponseInProgress       = "response.in_progress"
	ResponsesSSEResponseCompleted        = "response.completed"
	ResponsesSSEResponseFailed           = "response.failed"
	ResponsesSSEResponseIncomplete       = "response.incomplete"
	ResponsesSSEOutputItemAdded          = "response.output_item.added"
	ResponsesSSEOutputItemDone           = "response.output_item.done"
	ResponsesSSEContentPartAdded         = "response.content_part.added"
	ResponsesSSEContentPartDone          = "response.content_part.done"
	ResponsesSSETextDelta                = "response.output_text.delta"
	ResponsesSSETextDone                = "response.output_text.done"
	ResponsesSSERefusalDelta            = "response.refusal.delta"
	ResponsesSSEFunctionCallArgumentsDelta = "response.function_call_arguments.delta"
	ResponsesSSECodeInterpreterOutput    = "response.code_interpreter_call.output.delta"
	ResponsesSSEWebSearchCompleted      = "response.web_search_call.completed"
	ResponsesSSEFileSearchCallCompleted  = "response.file_search_call.completed"
	ResponsesSSEComputerCallOutput       = "response.computer_call_output.delta"
)
