package executor

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/just4zeroq/Omni-link/translator"
)

const (
	deepSeekOpenAI = "https://api.deepseek.com"
	deepSeekClaude = "https://api.deepseek.com/anthropic"
	deepSeekModel  = "deepseek-chat"
)

func deepSeekKey(t *testing.T) string {
	t.Helper()
	k := os.Getenv("DEEPSEEK_API_KEY")
	if k == "" {
		t.Skip("DEEPSEEK_API_KEY not set")
	}
	return k
}

var httpClient = &http.Client{}

func TestDeepSeekOpenAI(t *testing.T) {
	key := deepSeekKey(t)
	body := map[string]any{
		"model":    deepSeekModel,
		"messages": []map[string]any{{"role": "user", "content": "Say hello in one word."}},
		"stream":   false,
	}
	status, resp := doPost(t, deepSeekOpenAI+"/v1/chat/completions", body, "Bearer "+key)
	if status != 200 {
		t.Fatalf("status %d: %s", status, string(resp))
	}
	var m map[string]any
	mustUnmarshal(t, resp, &m)
	c, _ := m["choices"].([]any)
	if len(c) == 0 {
		t.Fatal("no choices")
	}
	t.Logf("OpenAI: %s", c[0].(map[string]any)["message"].(map[string]any)["content"])
}

func TestDeepSeekClaude(t *testing.T) {
	key := deepSeekKey(t)
	body := map[string]any{
		"model":      deepSeekModel,
		"messages":   []map[string]any{{"role": "user", "content": "Say hello in one word."}},
		"max_tokens": 4096,
		"stream":     false,
	}
	status, resp := doPost(t, deepSeekClaude+"/v1/messages", body, key)
	if status != 200 {
		t.Fatalf("status %d: %s", status, string(resp))
	}
	var m map[string]any
	mustUnmarshal(t, resp, &m)
	c, _ := m["content"].([]any)
	if len(c) == 0 {
		t.Fatal("no content")
	}
	t.Logf("Claude: %s", c[0].(map[string]any)["text"])
}

func TestDeepSeekOpenAIDirect(t *testing.T) {
	key := deepSeekKey(t)
	info := &RequestInfo{
		RequestID: "test-oa", RelayMode: translator.RelayModeChatCompletions,
		Model: deepSeekModel, ActualModelName: deepSeekModel,
		InboundFormat: translator.FormatOpenAI, ClientFormat: translator.FormatOpenAI,
		ApiKey: key, BaseURL: deepSeekOpenAI,
	}
	b, _ := json.Marshal(map[string]any{
		"model": deepSeekModel,
		"messages": []map[string]any{{"role": "user", "content": "Count 1 to 3."}},
		"stream": false,
	})
	resp, err := execReq(GetByProvider("deepseek"), info, b)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	mustUnmarshal(t, resp, &m)
	if _, ok := m["choices"]; !ok {
		t.Fatalf("no choices: %s", string(resp))
	}
	t.Logf("OA direct: %s", string(resp))
}

func TestDeepSeekClaudeDirect(t *testing.T) {
	key := deepSeekKey(t)
	info := &RequestInfo{
		RequestID: "test-cl", RelayMode: translator.RelayModeClaudeMessages,
		Model: deepSeekModel, ActualModelName: deepSeekModel,
		InboundFormat: translator.FormatClaude, ClientFormat: translator.FormatClaude,
		ApiKey: key, BaseURL: deepSeekOpenAI,
	}
	b, _ := json.Marshal(map[string]any{
		"model": deepSeekModel,
		"messages": []map[string]any{{"role": "user", "content": "Count 1 to 3."}},
		"max_tokens": 4096, "stream": false,
	})
	resp, err := execReq(GetByProvider("deepseek"), info, b)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	mustUnmarshal(t, resp, &m)
	if _, ok := m["content"]; !ok {
		t.Fatalf("no content: %s", string(resp))
	}
	t.Logf("Claude direct: %s", string(resp))
}

func TestDeepSeekConvOpenAIToClaude(t *testing.T) {
	key := deepSeekKey(t)
	info := &RequestInfo{
		RequestID: "test-conv", RelayMode: translator.RelayModeClaudeMessages,
		Model: deepSeekModel, ActualModelName: deepSeekModel,
		InboundFormat: translator.FormatOpenAI, ClientFormat: translator.FormatOpenAI,
		ApiKey: key, BaseURL: deepSeekOpenAI,
	}
	b, _ := json.Marshal(map[string]any{
		"model": deepSeekModel,
		"messages": []map[string]any{{"role": "user", "content": "Hi"}},
		"stream": false,
	})
	resp, err := execReq(GetByProvider("deepseek"), info, b)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	mustUnmarshal(t, resp, &m)
	if _, ok := m["choices"]; !ok {
		t.Fatalf("no choices: %s", string(resp))
	}
	t.Logf("Conv: %s", string(resp))
}

// ─── helpers ───

func doPost(t *testing.T, url string, body any, auth string) (int, []byte) {
	t.Helper()
	b, _ := json.Marshal(body)
	req, _ := http.NewRequest("POST", url, bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	if strings.HasPrefix(auth, "Bearer ") {
		req.Header.Set("Authorization", auth)
	} else {
		req.Header.Set("x-api-key", auth)
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, data
}

func execReq(e Executor, info *RequestInfo, body []byte) ([]byte, error) {
	up := translator.FormatOpenAI
	if info.RelayMode == translator.RelayModeClaudeMessages {
		up = translator.FormatClaude
	}
	conv, err := e.ConvertRequest(body, info.InboundFormat, up)
	if err != nil {
		return nil, err
	}
	conv = e.RequestCustomize(conv, info)
	resp, err := e.DoRequest(info, bytes.NewReader(conv))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return nil, nil
	}
	respBody = e.ResponseCustomize(respBody, info)
	return e.ConvertResponse(respBody, up, info.ClientFormat)
}

func mustUnmarshal(t *testing.T, data []byte, v any) {
	t.Helper()
	if err := json.Unmarshal(data, &v); err != nil {
		t.Fatalf("json: %v (body: %s)", err, string(data))
	}
}
