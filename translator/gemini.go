package translator

import "encoding/json"

// ========== Gemini API Types ==========

// GeminiChatRequest Gemini Chat API request body.
type GeminiChatRequest struct {
	Contents          []GeminiContent         `json:"contents"`
	SafetySettings    []GeminiSafetySetting   `json:"safetySettings,omitempty"`
	GenerationConfig  *GeminiGenerationConfig `json:"generationConfig,omitempty"`
	Tools             json.RawMessage         `json:"tools,omitempty"`
	ToolConfig        any                     `json:"toolConfig,omitempty"`
	SystemInstruction *GeminiContent          `json:"systemInstruction,omitempty"`
	CachedContent     string                  `json:"cachedContent,omitempty"`
	ServiceTier       string                  `json:"serviceTier,omitempty"`
}

// GeminiContent represents a Gemini content entry (role + parts).
type GeminiContent struct {
	Role  string       `json:"role"`
	Parts []GeminiPart `json:"parts"`
}

// GeminiPart is a union type for all Gemini content part variants.
type GeminiPart struct {
	Text                string                     `json:"text,omitempty"`
	InlineData          *GeminiInlineData          `json:"inlineData,omitempty"`
	FileData            *GeminiFileData            `json:"fileData,omitempty"`
	FunctionCall        *GeminiFunctionCall        `json:"functionCall,omitempty"`
	FunctionResponse    *GeminiFunctionResponse    `json:"functionResponse,omitempty"`
	Thought             *bool                      `json:"thought,omitempty"`
	ExecutableCode      *GeminiExecutableCode      `json:"executableCode,omitempty"`
	CodeExecutionResult *GeminiCodeExecutionResult `json:"codeExecutionResult,omitempty"`
}

// GeminiInlineData for images/audio embedded in requests.
type GeminiInlineData struct {
	MimeType string `json:"mimeType"`
	Data     string `json:"data"`
}

// GeminiFileData references a uploaded file URI.
type GeminiFileData struct {
	MimeType string `json:"mimeType,omitempty"`
	FileURI  string `json:"fileUri,omitempty"`
}

// GeminiFunctionCall represents a function call invocation.
type GeminiFunctionCall struct {
	FunctionName string `json:"name"`
	Arguments    any    `json:"args,omitempty"`
}

// GeminiFunctionResponse represents a function call result.
type GeminiFunctionResponse struct {
	Name     string `json:"name"`
	Response any    `json:"response,omitempty"`
}

// GeminiExecutableCode for code execution.
type GeminiExecutableCode struct {
	Language string `json:"language"`
	Code     string `json:"code"`
}

// GeminiCodeExecutionResult for code execution output.
type GeminiCodeExecutionResult struct {
	Outcome string `json:"outcome"`
	Output  string `json:"output"`
}

// GeminiGenerationConfig controls generation parameters.
type GeminiGenerationConfig struct {
	Temperature        *float64              `json:"temperature,omitempty"`
	TopP               *float64              `json:"topP,omitempty"`
	TopK               *float64              `json:"topK,omitempty"`
	MaxOutputTokens    *uint                 `json:"maxOutputTokens,omitempty"`
	CandidateCount     *int                  `json:"candidateCount,omitempty"`
	StopSequences      []string              `json:"stopSequences,omitempty"`
	ResponseMimeType   string                `json:"responseMimeType,omitempty"`
	ResponseSchema     any                   `json:"responseSchema,omitempty"`
	PresencePenalty    *float64              `json:"presencePenalty,omitempty"`
	FrequencyPenalty   *float64              `json:"frequencyPenalty,omitempty"`
	Seed               *int64                `json:"seed,omitempty"`
	ResponseLogprobs   *bool                 `json:"responseLogprobs,omitempty"`
	Logprobs           *int                  `json:"logprobs,omitempty"`
	ThinkingConfig     *GeminiThinkingConfig `json:"thinkingConfig,omitempty"`
	ResponseModalities []string              `json:"responseModalities,omitempty"`
}

// GeminiThinkingConfig for thinking/reasoning.
type GeminiThinkingConfig struct {
	IncludeThoughts bool   `json:"includeThoughts"`
	ThoughtBudget   *int   `json:"thoughtBudget,omitempty"`
	ThinkingLevel   string `json:"thinkingLevel,omitempty"`
}

// GeminiSafetySetting for content filtering.
type GeminiSafetySetting struct {
	Category  string `json:"category"`
	Threshold string `json:"threshold"`
}

// GeminiChatResponse Gemini API response.
type GeminiChatResponse struct {
	Candidates     []GeminiCandidate     `json:"candidates,omitempty"`
	PromptFeedback *GeminiPromptFeedback `json:"promptFeedback,omitempty"`
	UsageMetadata  *GeminiUsageMetadata  `json:"usageMetadata,omitempty"`
	ModelName      string                `json:"modelVersion,omitempty"`
}

// GeminiCandidate is a single response candidate.
type GeminiCandidate struct {
	Content           *GeminiContent `json:"content,omitempty"`
	FinishReason      string         `json:"finishReason,omitempty"`
	Index             int            `json:"index,omitempty"`
	SafetyRatings     []any          `json:"safetyRatings,omitempty"`
	GroundingMetadata any            `json:"groundingMetadata,omitempty"`
	AvgLogprobs       *float64       `json:"avgLogprobs,omitempty"`
}

// GeminiPromptFeedback contains safety feedback.
type GeminiPromptFeedback struct {
	SafetyRatings []any  `json:"safetyRatings,omitempty"`
	BlockReason   string `json:"blockReason,omitempty"`
}

// GeminiUsageMetadata token usage from Gemini.
type GeminiUsageMetadata struct {
	PromptTokenCount        int    `json:"promptTokenCount"`
	CandidatesTokenCount    int    `json:"candidatesTokenCount"`
	TotalTokenCount         int    `json:"totalTokenCount"`
	CachedContentTokenCount int    `json:"cachedContentTokenCount,omitempty"`
	ThoughtsTokenCount      int    `json:"thoughtsTokenCount,omitempty"`
}
