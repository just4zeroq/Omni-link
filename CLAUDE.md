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

**Conversions supported** (request + response):
- Claude ↔ OpenAI (bidirectional, full fidelity)
- OpenAI ↔ OpenAI Responses (bidirectional)
- Claude ↔ OpenAI Responses (bidirectional)
- Gemini ↔ OpenAI (via Gemini executor)

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
- Package-level prefix disambiguation: `cl`/`oa`/`ds` for executor-specific helpers, no prefix for translator package
- Interface-accepting, struct-returning for channel abstraction (channel typed as `any`)
- Pointer-heavy for optional fields (`*int`, `*string`, `*float64`) — zero-value = unset
- `json.RawMessage` for passthrough/raw fields (tools, tool_choice)

**Conversion naming:**
- `translator/conv.go`: canonical conversion functions (unprefixed, package-level)
- `executor/claude.go`: `cl`-prefixed Claude-specific conversions (claude.go implements Claude-specific converters that the translator package doesn't cover)
- `executor/openai.go`: `oa`-prefixed OpenAI-specific
- `executor/deepseek.go`: `ds`-prefixed DeepSeek-specific
- Each executor's ConvertRequest/ConvertResponse delegates to its own prefixed helpers

**Error patterns:**
- All conversions return `fmt.Errorf("provider: direction: %w", err)` — wraps with direction context
- Unsupported format pairs return explicit error, never silent fallthrough
- SSE parsing skips malformed events (continue), never fails the entire stream

**Import paths:**
`ai-platform/pkg/translator` (importing requires `translator.Format`, not `translator.Translator`)

## Testing

Not yet. Test files should mirror production structure: `translator/conv_test.go`, `executor/claude_test.go`, etc.
Focus areas: round-trip conversions (OpenAI→Claude→OpenAI idempotency), edge cases (empty content, tool calls, thinking blocks), SSE streaming fidelity.

## Common operations

- **Add new provider executor**: create `executor/<name>.go` with `init()` Registration, implement `Executor` interface, add suffix-prefixed conversion helpers
- **Add new format**: define types in new `translator/<name>.go`, add `Format` constant, implement `convertDirect` cases in `conv.go`
- **Add new channel mapping**: add `ProviderType` constant in `model/model.go`, add `ResolveProtocol` case
