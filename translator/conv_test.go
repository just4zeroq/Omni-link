package translator

import (
	"encoding/json"
	"testing"
)

// Sample bodies for all formats.

var openaiChatReq = []byte(`{"model":"deepseek-chat","messages":[{"role":"user","content":"Hello"}],"stream":false}`)

var openaiChatResp = []byte(`{"id":"chatcmpl-123","object":"chat.completion","created":1718000000,"model":"deepseek-chat","choices":[{"index":0,"message":{"role":"assistant","content":"Hi there!"},"finish_reason":"stop"}],"usage":{"prompt_tokens":10,"completion_tokens":5,"total_tokens":15}}`)

var claudeReq = []byte(`{"model":"deepseek-chat","messages":[{"role":"user","content":"Hello"}],"max_tokens":4096}`)

var claudeResp = []byte(`{"id":"msg_123","type":"message","role":"assistant","content":[{"type":"text","text":"Hi there!"}],"model":"deepseek-chat","stop_reason":"end_turn","usage":{"input_tokens":10,"output_tokens":5}}`)

var responsesReq = []byte(`{"model":"deepseek-chat","input":"Hello","max_output_tokens":100}`)

var responsesResp = []byte(`{"id":"resp_123","object":"response","status":"completed","model":"deepseek-chat","output":[{"type":"message","id":"msg_123","status":"completed","role":"assistant","content":[{"type":"output_text","text":"Hi there!"}]}],"usage":{"input_tokens":10,"output_tokens":5,"total_tokens":15}}`)

var geminiReq = []byte(`{"contents":[{"role":"user","parts":[{"text":"Hello"}]}],"generationConfig":{}}`)

var geminiResp = []byte(`{"candidates":[{"content":{"parts":[{"text":"Hi there!"}],"role":"model"},"finishReason":"STOP"}],"usageMetadata":{"promptTokenCount":10,"candidatesTokenCount":5,"totalTokenCount":15}}`)

// ─── DetectFormat ───

func TestDetectFormat(t *testing.T) {
	tests := []struct {
		name string
		body []byte
		want Format
	}{
		{"openai chat request", openaiChatReq, FormatOpenAI},
		{"openai chat response", openaiChatResp, FormatOpenAI},
		{"claude request", claudeReq, FormatClaude},
		{"claude response", claudeResp, FormatClaude},
		{"responses request", responsesReq, FormatOpenAIResponses},
		{"responses response", responsesResp, FormatOpenAIResponses},
		{"gemini request", geminiReq, FormatGemini},
		{"gemini response", geminiResp, FormatGemini},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DetectFormat(tt.body)
			if got != tt.want {
				t.Errorf("DetectFormat() = %v, want %v", got, tt.want)
			}
		})
	}
}

// ─── Conversion helpers ───

type convTest struct {
	name  string
	from  Format
	to    Format
	body  []byte
	check func(t *testing.T, in, out []byte)
}

func requireField(t *testing.T, body []byte, field string) {
	var m map[string]any
	if err := json.Unmarshal(body, &m); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	if _, ok := m[field]; !ok {
		t.Errorf("result missing field %q", field)
	}
}

func runConv(t *testing.T, tests []convTest) {
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Convert(tt.body, tt.from, tt.to)
			if err != nil {
				t.Fatalf("Convert(%s→%s): %v", tt.from, tt.to, err)
			}
			if len(got) == 0 {
				t.Fatal("empty result")
			}
			if tt.check != nil {
				tt.check(t, tt.body, got)
			}
		})
	}
}

// ─── All conversion pairs ───

func TestOpenAIToClaude(t *testing.T) {
	runConv(t, []convTest{
		{"request", FormatOpenAI, FormatClaude, openaiChatReq, func(t *testing.T, in, out []byte) {
			requireField(t, out, "max_tokens")
			requireField(t, out, "messages")
		}},
		{"response", FormatOpenAI, FormatClaude, openaiChatResp, func(t *testing.T, in, out []byte) {
			requireField(t, out, "content")
		}},
	})
}

func TestClaudeToOpenAI(t *testing.T) {
	runConv(t, []convTest{
		{"request", FormatClaude, FormatOpenAI, claudeReq, func(t *testing.T, in, out []byte) {
			requireField(t, out, "messages")
		}},
		{"response", FormatClaude, FormatOpenAI, claudeResp, func(t *testing.T, in, out []byte) {
			requireField(t, out, "choices")
		}},
	})
}

func TestOpenAIToResponses(t *testing.T) {
	runConv(t, []convTest{
		{"response", FormatOpenAI, FormatOpenAIResponses, openaiChatResp, func(t *testing.T, in, out []byte) {
			requireField(t, out, "output")
			requireField(t, out, "object")
		}},
	})
}

func TestResponsesToOpenAI(t *testing.T) {
	runConv(t, []convTest{
		{"request", FormatOpenAIResponses, FormatOpenAI, responsesReq, func(t *testing.T, in, out []byte) {
			requireField(t, out, "messages")
		}},
		{"response", FormatOpenAIResponses, FormatOpenAI, responsesResp, nil},
	})
}

func TestClaudeToResponses(t *testing.T) {
	runConv(t, []convTest{
		{"response", FormatClaude, FormatOpenAIResponses, claudeResp, func(t *testing.T, in, out []byte) {
			requireField(t, out, "output")
		}},
	})
}

func TestResponsesToClaude(t *testing.T) {
	runConv(t, []convTest{
		{"request", FormatOpenAIResponses, FormatClaude, responsesReq, func(t *testing.T, in, out []byte) {
			requireField(t, out, "max_tokens")
			requireField(t, out, "messages")
		}},
	})
}

func TestGeminiToOpenAI(t *testing.T) {
	runConv(t, []convTest{
		{"response", FormatGemini, FormatOpenAI, geminiResp, func(t *testing.T, in, out []byte) {
			requireField(t, out, "choices")
		}},
	})
}

func TestOpenAIToGemini(t *testing.T) {
	runConv(t, []convTest{
		{"request", FormatOpenAI, FormatGemini, openaiChatReq, func(t *testing.T, in, out []byte) {
			requireField(t, out, "contents")
		}},
	})
}

func TestClaudeToGemini(t *testing.T) {
	runConv(t, []convTest{
		{"request", FormatClaude, FormatGemini, claudeReq, func(t *testing.T, in, out []byte) {
			requireField(t, out, "contents")
		}},
	})
}

func TestGeminiToClaude(t *testing.T) {
	runConv(t, []convTest{
		{"response", FormatGemini, FormatClaude, geminiResp, func(t *testing.T, in, out []byte) {
			requireField(t, out, "content")
		}},
	})
}

func TestResponsesToGemini(t *testing.T) {
	runConv(t, []convTest{
		{"request", FormatOpenAIResponses, FormatGemini, responsesReq, func(t *testing.T, in, out []byte) {
			requireField(t, out, "contents")
		}},
	})
}

func TestGeminiToResponses(t *testing.T) {
	runConv(t, []convTest{
		{"response", FormatGemini, FormatOpenAIResponses, geminiResp, func(t *testing.T, in, out []byte) {
			requireField(t, out, "output")
		}},
	})
}

func TestOpenAIToClaudeRoundTrip(t *testing.T) {
	mid, err := Convert(openaiChatReq, FormatOpenAI, FormatClaude)
	if err != nil {
		t.Fatalf("openai→claude: %v", err)
	}
	back, err := Convert(mid, FormatClaude, FormatOpenAI)
	if err != nil {
		t.Fatalf("claude→openai: %v", err)
	}
	var result map[string]any
	mustUnmarshal(t, back, &result)
	if _, ok := result["messages"]; !ok {
		t.Error("round-trip missing messages")
	}
}

func mustUnmarshal(t *testing.T, data []byte, v any) {
	t.Helper()
	if err := json.Unmarshal(data, &v); err != nil {
		t.Fatalf("json unmarshal: %v (body: %s)", err, string(data))
	}
}
