package translator

import "encoding/json"

// ========== OpenAI Responses API 请求 ==========

// ResponsesRequest OpenAI /v1/responses 请求
// 参考 OpenAI-Responses协议文档 §2.1 请求体参数
type ResponsesRequest struct {
	Model                string          `json:"model"`
	Input                json.RawMessage `json:"input,omitempty"`           // string | []InputItem
	Instructions         json.RawMessage `json:"instructions,omitempty"`
	Tools                json.RawMessage `json:"tools,omitempty"`
	ToolChoice           json.RawMessage `json:"tool_choice,omitempty"`
	Temperature          *float64        `json:"temperature,omitempty"`
	TopP                 *float64        `json:"top_p,omitempty"`
	MaxOutputTokens      *uint           `json:"max_output_tokens,omitempty"`
	MaxToolCalls         *uint           `json:"max_tool_calls,omitempty"`
	Stream               *bool           `json:"stream,omitempty"`
	StreamOptions        *StreamOptions  `json:"stream_options,omitempty"`
	Store                *bool           `json:"store,omitempty"`
	Reasoning            *ReasoningConfig `json:"reasoning,omitempty"`
	Text                 json.RawMessage `json:"text,omitempty"`
	PreviousResponseID   string          `json:"previous_response_id,omitempty"`
	Conversation         json.RawMessage `json:"conversation,omitempty"`
	Background           *bool           `json:"background,omitempty"`
	Include              json.RawMessage `json:"include,omitempty"`
	ParallelToolCalls    *bool           `json:"parallel_tool_calls,omitempty"`
	Truncation           string          `json:"truncation,omitempty"`      // "auto" | "disabled"
	Metadata             map[string]any  `json:"metadata,omitempty"`
	User                 string          `json:"user,omitempty"`
	PromptCacheKey       string          `json:"prompt_cache_key,omitempty"`
	PromptCacheRetention string          `json:"prompt_cache_retention,omitempty"`
	SafetyIdentifier     string          `json:"safety_identifier,omitempty"`
	ServiceTier          string          `json:"service_tier,omitempty"`
	Prompt               json.RawMessage `json:"prompt,omitempty"`
	Logprobs             *int            `json:"logprobs,omitempty"`
}

// ReasoningConfig 推理参数
type ReasoningConfig struct {
	Effort  string `json:"effort,omitempty"`  // "none" | "low" | "medium" | "high"
	Summary string `json:"summary,omitempty"`
}

// ========== 输入项类型 ==========

// InputItem 输入项（所有输入类型的联合）
type InputItem struct {
	Type      string `json:"type"`
	Text      string `json:"text,omitempty"`
	ImageURL  string `json:"image_url,omitempty"`
	FileURL   string `json:"file_url,omitempty"`
	FileID    string `json:"file_id,omitempty"`
	Filename  string `json:"filename,omitempty"`
	Content   string `json:"content,omitempty"`
	Detail    string `json:"detail,omitempty"`
	ID        string `json:"id,omitempty"`
	Role      string `json:"role,omitempty"`
	Status    string `json:"status,omitempty"`
	CallID    string `json:"call_id,omitempty"`
	Output    any    `json:"output,omitempty"`
	Approve   *bool  `json:"approve,omitempty"`
	Reason    string `json:"reason,omitempty"`
}

// ========== 输出项类型 ==========

// ResponsesOutput 响应输出项（所有 16+ 输出类型的联合体）
type ResponsesOutput struct {
	Type      string                   `json:"type"`
	ID        string                   `json:"id"`
	Status    string                   `json:"status,omitempty"`
	Role      string                   `json:"role,omitempty"`
	Content   []ResponsesOutputContent `json:"content,omitempty"`
	CallID    string                   `json:"call_id,omitempty"`
	Name      string                   `json:"name,omitempty"`
	Arguments string                   `json:"arguments,omitempty"`

	// message 字段
	Annotations []any `json:"annotations,omitempty"`

	// web_search_call
	Action any `json:"action,omitempty"`

	// file_search_call
	Queries []string `json:"queries,omitempty"`
	Results []any    `json:"results,omitempty"`

	// computer_call
	PendingSafetyChecks []any `json:"pending_safety_checks,omitempty"`

	// reasoning
	Summary          []any  `json:"summary,omitempty"`
	EncryptedContent string `json:"encrypted_content,omitempty"`

	// image_generation_call
	Result string `json:"result,omitempty"`

	// code_interpreter_call
	Code         string `json:"code,omitempty"`
	ContainerID  string `json:"container_id,omitempty"`
	Outputs      []any  `json:"outputs,omitempty"`

	// mcp_call
	ServerLabel       string `json:"server_label,omitempty"`
	Error             string `json:"error,omitempty"`
	ApprovalRequestID string `json:"approval_request_id,omitempty"`

	// mcp_list_tools
	Tools []any `json:"tools,omitempty"`

	// 嵌套 action 字段
	Command            []string `json:"command,omitempty"`
	MaxOutputLength    *int     `json:"max_output_length,omitempty"`
	Timeout            *int     `json:"timeout,omitempty"`
	Path               string   `json:"path,omitempty"`
	Patch              string   `json:"patch,omitempty"`
}

// ActionType 操作类型用于 shell_call / computer_call / apply_patch_call
type ActionType string

const (
	ActionExec      ActionType = "exec"
	ActionClick    ActionType = "click"
	ActionTypeStr  ActionType = "type"
	ActionScreenshot ActionType = "screenshot"
)

// ResponsesOutputContent 输出内容子项（message 类型内部）
type ResponsesOutputContent struct {
	Type        string       `json:"type"`                  // "output_text" | "refusal" | ...
	Text        string       `json:"text,omitempty"`
	Refusal     string       `json:"refusal,omitempty"`
	Annotations []any        `json:"annotations,omitempty"`
}

// ========== 非流式响应 ==========

// ResponsesResponse OpenAI /v1/responses 非流式响应
type ResponsesResponse struct {
	ID                   string              `json:"id"`
	Object               string              `json:"object"`            // "response"
	CreatedAt            int64               `json:"created_at"`
	CompletedAt          int64               `json:"completed_at,omitempty"`
	Status               string              `json:"status"`            // "completed" | "failed" | "in_progress" | ...
	Error                any                 `json:"error"`
	IncompleteDetails    any                 `json:"incomplete_details"`
	Instructions         any                 `json:"instructions"`
	MaxOutputTokens      *int                `json:"max_output_tokens"`
	MaxToolCalls         *int                `json:"max_tool_calls,omitempty"`
	Model                string              `json:"model"`
	Output               []ResponsesOutput   `json:"output"`
	OutputText           string              `json:"output_text,omitempty"`
	ParallelToolCalls    bool                `json:"parallel_tool_calls"`
	PreviousResponseID   any                 `json:"previous_response_id"`
	Prompt               any                 `json:"prompt"`
	PromptCacheKey       string              `json:"prompt_cache_key,omitempty"`
	PromptCacheRetention string              `json:"prompt_cache_retention,omitempty"`
	Reasoning            *ResponsesReasoning `json:"reasoning"`
	SafetyIdentifier     string              `json:"safety_identifier,omitempty"`
	ServiceTier          string              `json:"service_tier,omitempty"`
	Store                bool                `json:"store"`
	Background           *bool               `json:"background,omitempty"`
	Conversation         any                 `json:"conversation"`
	Temperature          *float64            `json:"temperature"`
	Text                 *ResponsesTextObj   `json:"text,omitempty"`
	ToolChoice           any                 `json:"tool_choice"`
	Tools                []any               `json:"tools"`
	TopP                 *float64            `json:"top_p"`
	Truncation           any                 `json:"truncation"`
	Usage                *ResponsesUsage     `json:"usage,omitempty"`
	User                 any                 `json:"user"`
	Metadata             any                 `json:"metadata"`
}

// ResponsesReasoning 响应中的推理字段
type ResponsesReasoning struct {
	Effort  any `json:"effort"`
	Summary any `json:"summary"`
}

// ResponsesTextObj 文本格式配置（响应中的 text 字段）
type ResponsesTextObj struct {
	Format ResponsesTextFormat `json:"format"`
}

// ResponsesTextFormat 文本格式
type ResponsesTextFormat struct {
	Type string `json:"type"` // "text" | "json_object" | "json_schema"
}

// ResponsesUsage Responses API 用量
type ResponsesUsage struct {
	InputTokens        int                  `json:"input_tokens"`
	OutputTokens       int                  `json:"output_tokens"`
	TotalTokens        int                  `json:"total_tokens"`
	InputTokensDetails *InputTokenDetails   `json:"input_tokens_details,omitempty"`
	OutputTokenDetails *OutputTokenDetails  `json:"output_tokens_details,omitempty"`
}

// InputTokenDetails 输入 token 细分
type InputTokenDetails struct {
	CachedTokens int `json:"cached_tokens,omitempty"`
	TextTokens   int `json:"text_tokens,omitempty"`
	AudioTokens  int `json:"audio_tokens,omitempty"`
	ImageTokens  int `json:"image_tokens,omitempty"`
}

// OutputTokenDetails 输出 token 细分
type OutputTokenDetails struct {
	TextTokens               int `json:"text_tokens,omitempty"`
	AudioTokens              int `json:"audio_tokens,omitempty"`
	ReasoningTokens          int `json:"reasoning_tokens,omitempty"`
	AcceptedPredictionTokens int `json:"accepted_prediction_tokens,omitempty"`
	RejectedPredictionTokens int `json:"rejected_prediction_tokens,omitempty"`
}
