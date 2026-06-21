# Omni-link — LLM API Protocol Translation Layer

Go module bridging LLM API protocol formats (OpenAI Chat, Claude Messages, OpenAI Responses, Gemini). Client using protocol A can transparently call provider of protocol B.

## Architecture

```
model/       → Provider types, channel config, protocol metadata
translator/  → Format conversion engine + protocol type definitions
executor/    → Provider-specific executors with plugin registry
```

### model — Type definitions

- `ProviderType`: 40+ provider enum (OpenAI=1, Claude=2, Gemini=3, DeepSeek=8, ...)
- `ProtocolType`: upstream API protocol ("openai-compatible", "anthropic-compatible", ...)
- `Channel`: llm_channels row with protocols, API key, settings
- `ResolveProtocol(ProviderType)`: maps provider → default protocol

### translator — Format conversion

Core conversion engine. 4 supported formats:

| Format | Endpoint | Default Relay Mode |
|--------|----------|-------------------|
| `openai` | `/v1/chat/completions` | `chat-completions` |
| `claude` | `/v1/messages` | `claude-messages` |
| `openai_responses` | `/v1/responses` | `responses` |
| `gemini` | (Google endpoint) | — |

**Conversion matrix** (✓ = direct, · = via hub/OpenAI intermediate, — = same format):

| from → to | openai | claude | responses | gemini |
|-----------|--------|--------|-----------|--------|
| **openai** | — | ✓ | ✓ | ✓ |
| **claude** | ✓ | — | ✓ | ✓ |
| **responses** | ✓ | ✓ | — | ✓ |
| **gemini** | ✓ | ✓ | ✓ | — |

All 12 pairs direct. No hub fallback needed for any format combination.

**Key functions:**
- `Convert(body, from, to)` — entry point. Falls back via OpenAI intermediate when direct path missing
- `DetectFormat(body, path)` — format detection (path first, then body inspection)
- `DetectFormatFromPath(path)` — URL path → format lookup
- `DetectFormat(body)` — heuristics: has "input"? → Responses. Has "messages" + "max_tokens"? → Claude. Has "temperature"? → OpenAI. Default → OpenAI

All protocol type definitions live in `translator/`:
- `openai.go` — `ChatRequest`, `Message`, `Tool`, `ChatResponse`, `ChatStreamChunk`
- `claude.go` — `ClaudeRequest`, `ClaudeMessage`, `ClaudeResponse`, SSE event constants
- `responses.go` — `ResponsesRequest`, `InputItem`, `ResponsesOutput`, `ResponsesResponse`
- `gemini.go` — `GeminiChatRequest`, `GeminiContent`, `GeminiPart`, `GeminiChatResponse`, `GeminiThinkingConfig`

### executor — Provider execution

Each provider implements `Executor` interface and self-registers via `init()`:

```go
func init() { Register("claude", &ClaudeExecutor{}) }
```

**Executor interface:**
- `Init(channel)` — configure from `*model.Channel`
- `NativeFormats()` — upstream formats this executor natively supports
- `GetRequestURL(info)` — build upstream URL
- `SetupRequestHeader(header, info)` — auth, content-type, streaming headers
- `ConvertRequest/ConvertResponse(body, from, to)` — format conversion
- `RequestCustomize/ResponseCustomize(body, info)` — vendor-specific injection
- `NewResponseStream(from, to)` — SSE streaming converter
- `DoRequest(info, body)` — raw HTTP call

**Implemented executors:**
- `claude` — native Claude, SSE streaming Claude↔OpenAI
- `openai` — native OpenAI Chat, includes Responses↔OpenAI
- `gemini` — native Gemini, hub via OpenAI intermediate
- `deepseek` — dual native (OpenAI + Claude), custom thinking/reasoning injection
- `volcengine` — OpenAI-compatible, volcengine-specific URL routing

**Format planning:**
`Plan(input, output, capabilities)` selects optimal upstream format minimizing conversions (score = input mismatch + output mismatch). Prefers format matching output format on tie.

**SSE streaming architecture:**
Streaming conversion uses `ResponseStream` interface with `Feed(chunk)` / `End()` / `Usage()` methods. Implemented for Claude ↔ OpenAI (both directions) via stateful stream converters:
- `claudeToOpenAIStream` — maps Claude SSE events (message_start, content_block_start/delta/stop, message_delta/stop) → OpenAI `data:` chunks
- `openAIToClaudeStream` — maps OpenAI `data:` chunks → Claude SSE events

Both buffer incomplete events, handle tool calls, track usage accumulation.

## Conventions

**Code style:**
- Pointer-heavy for optional fields (`*int`, `*string`, `*float64`) — zero-value = unset
- `json.RawMessage` for passthrough/raw fields (tools, tool_choice)
- Channel typed as `any` for abstraction (never `*model.Channel` directly)

**Conversion architecture (SINGLE SOURCE OF TRUTH):**
- All format conversion logic lives in `translator/conv.go` — canonical, unprefixed
- All executors delegate ConvertRequest/ConvertResponse to `translator.Convert(body, from, to)`
- No conversion code duplication across executor files
- Vendor-specific modifications go in `RequestCustomize`/`ResponseCustomize` hooks (e.g. `dsInjectThinking`, `injectStreamOptionsOpenAI`, `replaceModelField`)

**Exception:** Gemini executor keeps its own conversion logic (Gemini format not yet in translator). Planned for consolidation.

**Error patterns:**
- All conversions return `fmt.Errorf("provider: direction: %w", err)` — wraps with direction context
- Unsupported format pairs return explicit error, never silent fallthrough
- SSE parsing skips malformed events (continue), never fails the entire stream

**Import paths:**
`github.com/just4zeroq/Omni-link/translator`

## Testing

**Unit tests** (`translator/conv_test.go`):
- 37 test cases covering all 12 format conversion pairs + round-trip + DetectFormat
- Run: `go test ./translator/`

**Integration tests** (`executor/deepseek/` + `executor/volcengine/`):
Requires valid API keys in `.env`. DeepSeek tests cover:
- OpenAI-compatible endpoint (`/v1/chat/completions`)
- Anthropic-compatible endpoint (`/anthropic/v1/messages`)
- Format conversion (OpenAI→Claude→OpenAI round-trip)
- Full executor pipeline, streaming, tools, thinking, error handling
- Run: `go test ./executor/deepseek/ -timeout 120s`

Volcengine (Doubao/火山引擎) tests cover:
- OpenAI Chat + Responses API endpoints
- Streaming (both Chat + Responses SSE passthrough)
- Format conversion (Responses↔Chat via Plan)
- System message, tools, params, error handling
- Run: `go test ./executor/volcengine/ -timeout 120s`

**DeepSeek API**:
- OpenAI format: `https://api.deepseek.com/v1/chat/completions` (auth: `Authorization: Bearer`)
- Claude format: `https://api.deepseek.com/anthropic/v1/messages` (auth: `x-api-key`)
- `UpstreamFormat` controls auth header and URL path selection
- Notable: `deepseek-chat` model resolves to `deepseek-v4-flash` upstream

## Common operations

- **Add new provider executor**: create `executor/<name>.go` with `init()` Registration, implement `Executor` interface, add suffix-prefixed conversion helpers
- **Add new format**: define types in new `translator/<name>.go`, add `Format` constant, implement `convertDirect` cases in `conv.go`
- **Add new channel mapping**: add `ProviderType` constant in `model/model.go`, add `ResolveProtocol` case
