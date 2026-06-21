# Omni-link

**Universal AI Protocol Translation Library** — embeddable format conversion + provider execution layer for text, speech, image, and video AI APIs.

Client using any API format (OpenAI Chat, Claude Messages, OpenAI Responses, Gemini) can transparently call any provider, across any modality. Protocol translation, streaming conversion, and provider abstraction consolidated in a single embeddable library.

> **Status**: Text protocol translation layer complete. Speech/image/video provider framework in design phase.

---

## Modality Roadmap

| Modality | Status | Provider Types | Description |
|----------|--------|---------------|-------------|
| **Text** | ✅ Complete | OpenAI, Claude, Gemini, DeepSeek, Volcengine, plus 35+ provider types | 4-format conversion, streaming, tool calls, thinking |
| **Speech** (TTS/STT) | 🚧 Planned | — | Text-to-speech + speech-to-text unified interface |
| **Image** | 🚧 Planned | Midjourney, Jimeng, plus standard APIs | Image generation, editing, variation |
| **Video** | 🚧 Planned | Sora, Kling, plus standard APIs | Video generation, editing |

---

## Text Protocol Translation (Current)

### Supported Formats

| Format | Endpoint | Request Schema | Response Schema |
|--------|----------|---------------|----------------|
| `openai` | `/v1/chat/completions` | `messages` + tools | `choices` |
| `claude` | `/v1/messages` | `messages` + `max_tokens` | `type: "message"` |
| `openai_responses` | `/v1/responses` | `input` | `output` |
| `gemini` | (Google endpoint) | `contents` | `candidates` |

### Conversion Matrix — All 12 Pairs Direct

| from ↓ → to | openai | claude | responses | gemini |
|------------|--------|--------|-----------|--------|
| **openai** | — | ✓ | ✓ | ✓ |
| **claude** | ✓ | — | ✓ | ✓ |
| **responses** | ✓ | ✓ | — | ✓ |
| **gemini** | ✓ | ✓ | ✓ | — |

No intermediate hub needed. Fallback via OpenAI for any undirected pair.

---

## Architecture

```
┌────────────────────────────────────────────────────────────┐
│                         model/                              │
│  ProviderType (40+), Channel config, Protocol resolution   │
└─────────────────────────┬──────────────────────────────────┘
                          │
┌─────────────────────────▼──────────────────────────────────┐
│                      translator/                            │
│  Format detection + conversion engine                      │
│  Convert(body, from, to) → unified internal → to target    │
│  4 format definitions + 12 directional converters          │
│  (Text only; speech/image/video TBD)                        │
└─────────────────────────┬──────────────────────────────────┘
                          │
┌─────────────────────────▼──────────────────────────────────┐
│                      executor/                              │
│  Provider implementations + plugin registry                │
│  ┌──────────┬──────────┬──────────┬──────────┬──────────┐  │
│  │ Claude   │ OpenAI   │ Gemini   │ DeepSeek │Volcengine│  │
│  │ Text      │ Text      │ Text      │ Text ✓    │ Text ✓   │  │
│  │          │          │          │          │          │  │
│  │ TBD      │ TBD      │ TBD      │ TBD      │ TBD      │  │
│  └──────────┴──────────┴──────────┴──────────┴──────────┘  │
│  Plan() → optimal upstream format                           │
│  SSE streaming converters (Claude↔OpenAI)                  │
└─────────────────────────────────────────────────────────────┘
```

### Three-Layer Design

**model/** — Provider enumeration, channel configuration, protocol metadata
- 40+ `ProviderType` constants (text, image, video, audio)
- `Channel` struct with protocol list + API key binding
- `ResolveProtocol()` maps provider → default protocol

**translator/** — Format conversion engine (Text)
- `Convert(body, from, to)` — entry point for all Text format conversion
- `DetectFormat(body, path)` — format detection (path → body heuristics)
- 4 protocol type definition files + 12 directional converter functions
- Extensible: new formats add a file + `convertDirect` case

**executor/** — Provider execution layer with plugin registry
- `Executor` interface: `Init`, `NativeEndpoints`, `GetRequestURL`, `SetupRequestHeader`, `ConvertRequest`/`ConvertResponse`, `RequestCustomize`/`ResponseCustomize`, `NewResponseStream`, `DoRequest`
- `Register("name", &Executor{})` — self-registration via `init()`
- `Plan(input, output, endpoints)` — selects optimal upstream format (scoring: input_mismatch + output_mismatch, tie → output format)
- `RequestInfo.UpstreamFormat` — zero-value `""` triggers Plan; 4-level client override granularity

---

## Provider Implementations — Text

| Executor | Native Formats | Streaming | Integration Tests |
|----------|---------------|-----------|-------------------|
| **Claude** | `claude` (`/v1/messages`) | ✅ Claude↔OpenAI | translator-level |
| **OpenAI** | `openai` (`/v1/chat/completions`) | ✅ Native | translator-level |
| **Gemini** | `gemini` (Google endpoint) | ⚠️ Via OpenAI hub | translator-level |
| **DeepSeek** | `openai` + `claude` dual | ✅ Bidirectional | **27 tests** |
| **Volcengine** | `openai` + `openai_responses` dual | ✅ Native SSE | **32 tests** |

### DeepSeek
- Dual native endpoints (OpenAI `/v1/chat/completions` + Claude `/anthropic/v1/messages`)
- Auth: Bearer token for OpenAI path, `x-api-key` for Claude path
- Thinking/reasoning injection via `RequestCustomize`
- 27 tests: Chat, streaming, format conversion, Plan auto-resolve, tools, thinking, error handling

### Volcengine / Doubao (火山引擎)
- OpenAI Chat (`/api/v3/chat/completions`) + Responses (`/api/v3/responses`)
- Auth: `Authorization: Bearer` + model in body
- Multi-model tested: doubao-seed-2-0-lite, GLM-4-7B, DeepSeek V3
- Bot model routing (`bot-` prefix → `/api/v3/bots/chat/completions`)
- `stream_options: {"include_usage": true}` injection for Chat SSE
- 32 tests: Chat (3 models), Responses, streaming, 10-direction format conversion, Plan, tools, params, error

---

## Test Coverage — 96 Tests, All Passing ✅

```
translator/conv_test.go         37 tests   Format detection + 12 conversion pairs
executor/deepseek/              27 tests   Full DeepSeek pipeline
executor/volcengine/            32 tests   Full Volcengine pipeline
─────────────────────────────────────────
Total                            96 tests   go test ./... -count=1 -timeout 300s
```

### Per-Package

```bash
go test ./translator/                              # 37 — no API keys needed
go test ./executor/deepseek/ -timeout 120s          # 27 — requires DEEPSEEK_API_KEY
go test ./executor/volcengine/ -timeout 180s        # 32 — requires VOLC_API_KEY
```

Integration tests require `.env`:
```env
DEEPSEEK_API_KEY=sk-...
VOLC_API_KEY=ark-...
```

---

## Project Structure

```
Omni-link/
├── model/                  # [layer] Provider types, channel config
│   └── model.go            # 40+ ProviderType, Protocol resolution, Channel struct
├── translator/             # [layer] Format conversion engine (Text)
│   ├── translator.go       #   Entry: Convert, Detect, Format constants
│   ├── conv.go             #   12 directional converters + helpers
│   ├── conv_test.go        #   37 unit tests
│   ├── openai.go           #   OpenAI Chat type defs
│   ├── claude.go           #   Claude Messages type defs
│   ├── gemini.go           #   Gemini type defs
│   └── responses.go        #   Responses API type defs
├── executor/               # [layer] Provider implementations
│   ├── executor.go         #   Executor interface, RequestInfo, Plan()
│   ├── registry.go         #   Provider registry
│   ├── shared.go           #   Helpers (ReplaceModelField, etc.)
│   ├── stream_exec.go      #   Stream execution pipeline
│   ├── streams.go          #   SSE stream converters (Claude↔OpenAI)
│   ├── claude/             #   Anthropic Claude executor
│   ├── openai/             #   OpenAI Chat executor
│   ├── gemini/             #   Google Gemini executor
│   ├── deepseek/           #   DeepSeek executor (27 tests)
│   └── volcengine/         #   Volcengine/Doubao executor (32 tests)
├── CLAUDE.md               # Dev conventions
├── go.mod                  # Go 1.23, zero external deps
└── README.md
```

## Future Modalities — Design Direction

Each modality will follow the same three-layer pattern with modality-specific interfaces:

```
model/          → Adds modality tags to ProviderType
translator/     → modality/ sub-packages (text/, speech/, image/, video/)
executor/       → Modality-aware executors per provider
```

### Speech (TTS / STT)
- Input: text + voice params → Output: audio stream / file
- Providers: OpenAI TTS, Azure Speech, ElevenLabs, Suno
- Translation: SSML ↔ plain text, voice profile mapping

### Image Generation
- Input: prompt + params → Output: image URL / base64
- Providers: Midjourney, DALL-E, Stable Diffusion, Jimeng
- Translation: Prompt style normalization, parameter mapping

### Video Generation
- Input: prompt + params → Output: video URL / stream
- Providers: Sora, Kling, Runway

---

## Adding a New Provider

1. **Define `ProviderType`** in `model/model.go`
2. **Add format types** (if new protocol) in `translator/`
3. **Implement `Executor`** in `executor/<name>.go` with `init()` registration
4. **Define `NativeEndpoints()`** — supported formats + URL paths
5. **Add vendor logic** in `RequestCustomize`/`ResponseCustomize`
6. **Write integration tests** in `executor/<name>/<name>_test.go`
