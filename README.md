# Omni-link

**Universal AI Protocol Translation Library** — Go library bridging AI API formats across text, image, audio, and video.

```go
import "github.com/just4zeroq/Omni-link/translator"
import textexec "github.com/just4zeroq/Omni-link/executor/text"

// Transparent format conversion: OpenAI ↔ Claude ↔ Responses ↔ Gemini
result, _ := translator.Convert(openaiBody, translator.FormatOpenAI, translator.FormatClaude)

// Full executor pipeline with auto-format planning
resp, _ := textexec.Request(claudeExecutor, info, body)

// Streaming with cross-format SSE conversion
textexec.ExecuteStream(ctx, executor, info, body, callback)
```

[![Go Version](https://img.shields.io/badge/Go-1.23-00ADD8?style=flat-square&logo=go)](https://go.dev)
[![Tests](https://img.shields.io/badge/Tests-96_passing-22c55e?style=flat-square)](https://github.com/just4zeroq/Omni-link)
[![License](https://img.shields.io/badge/License-MIT-000000?style=flat-square)](LICENSE)
[![Zero Deps](https://img.shields.io/badge/Dependencies-Zero-6366f1?style=flat-square)](go.mod)

> **Status**: Text protocol translation ✅ | Image providers ✅ | Audio providers ✅ | Video providers ✅

---

## Modality Roadmap

| Category | Status | Provider Types |
|---|---|---|
| **🔤 Text** | ✅ Complete | OpenAI, Claude, Gemini, DeepSeek, Volcengine + 35+ more |
| **🖼️ Image** | ✅ Complete | GPT Image 2, Midjourney, Seedream, Qwen, Nano Banana, Z Image, Wan2.5 |
| **🎵 Audio** | ✅ Complete | OpenAI TTS/STT, ElevenLabs, Azure, PlayHT, Cartesia, Fish Audio, CosyVoice, FunASR, Suno |
| **🎬 Video** | ✅ Complete | Sora, Kling, Runway, Seedance, Hailuo, Pika, Wan, Luma, Grok, OmniHuman, HappyHorse |

---

## Quick Start

```bash
go get github.com/just4zeroq/Omni-link
```

```go
package main

import (
    "github.com/just4zeroq/Omni-link/translator"
    textexec "github.com/just4zeroq/Omni-link/executor/text"
)

func main() {
    // 1. Convert formats
    claudeReq := `{"messages":[{"role":"user","content":"Hello"}],"max_tokens":1024}`
    openaiReq, _ := translator.Convert([]byte(claudeReq),
        translator.FormatClaude, translator.FormatOpenAI)
    // openaiReq → {"model":"...","messages":[...],"max_tokens":1024}

    // 2. Use an executor
    e := &textexec.ClaudeExecutor{}
    e.Init(channel)

    info := &textexec.RequestInfo{
        InboundFormat:  translator.FormatOpenAI,
        ClientFormat:   translator.FormatOpenAI,
        UpstreamFormat: translator.FormatClaude, // auto-resolve via Plan()
        IsStream:       true,
    }
    textexec.ExecuteStream(ctx, e, info, body, callback)
}
```

### Image / Audio / Video Quick Start

```go
import (
    imageexec "github.com/just4zeroq/Omni-link/executor/image"
    audioexec "github.com/just4zeroq/Omni-link/executor/audio"
    videoexec "github.com/just4zeroq/Omni-link/executor/video"
)

// Image generation (7 providers)
imgExec, _ := imageexec.GetImage("gptimage")
result, _ := imgExec.TextToImage(&imageexec.TextToImageRequest{
    Prompt: "A cat wearing a hat", Model: "dall-e-3",
    N: 1, Size: "1024x1024",
})

// TTS with unified streaming (9 audio providers)
audioExec, _ := audioexec.GetAudio("cartesia")
stream, _ := audioExec.TextToSpeech(&audioexec.TTSRequest{
    Input: "Hello world",
    Voice: "a0e41e7a-6b41-4b50-9b09-64b0e0d717f5",
})
result, _ := stream.Collect() // or range stream.Chunk for streaming

// Video generation (11 providers, all async)
videoExec, _ := videoexec.GetVideo("kling")
task, _ := videoExec.TextToVideo(&videoexec.TextToVideoRequest{
    Prompt: "A rocket launching",
})
// Poll: videoExec.GetTask(task.ID)
```

---

## Text Protocol Translation

### Client-Exposed Formats

| Format | Endpoint | Schema |
|---|---|---|
| `OpenAI` | `/v1/chat/completions` | `messages` + tools → `choices` |
| `Claude` | `/v1/messages` | `messages` + `max_tokens` → `type: "message"` |
| `OpenAI Responses` | `/v1/responses` | `input` → `output` |

### Conversion Matrix — All 12 Pairs Covered

| from ↓ → to | openai | claude | responses | gemini |
|---|---|---|---|---|
| **openai** | — | ✓ | ✓ | ✓ |
| **claude** | ✓ | — | ✓ | ✓ |
| **responses** | ✓ | ✓ | — | ✓ |
| **gemini** ¹ | ✓ | ✓ | ✓ | — |

¹ Gemini format = internal only (Gemini executor). No direct client exposure.  
Unsupported pairs auto-fallback via OpenAI intermediate hub.

---

## Architecture

```
┌──────────────────────────────────────────────────────────────────┐
│                            model/                                 │
│  ProviderType (40+), Channel config, Protocol resolution         │
└────────────────────────────┬─────────────────────────────────────┘
                             │
┌────────────────────────────▼─────────────────────────────────────┐
│                         translator/                               │
│  Convert(body, from, to) — format detection + conversion engine  │
│  Type definitions: openai.go, claude.go, responses.go, gemini.go │
│  12 directional converters in conv.go                             │
└────────────────────────────┬─────────────────────────────────────┘
                             │
┌────────────────────────────▼─────────────────────────────────────┐
│                         executor/                                 │
│  ┌─────────────── text/ ─────────────────────────────────────┐    │
│  │ Claude │ OpenAI │ Gemini │ DeepSeek │ Volcengine          │    │
│  │ Cl     │ OAI    │ GEM    │ OAI+Cl   │ OAI+RSP            │    │
│  └───────────────────────────────────────────────────────────┘    │
│  ┌─────────────── image/ ──────────────────────────────────────┐  │
│  │ GPT Image │ Qwen │ NanoBanana │ ZImage │ Wan │ Seedream    │  │
│  │ Midjourney                                                  │  │
│  └───────────────────────────────────────────────────────────┘  │
│  ┌─────────────── audio/ ─────────────────────────────────────┐  │
│  │ OpenAI │ ElevenLabs │ CosyVoice │ Suno │ FunASR │ Azure   │  │
│  │ PlayHT │ Cartesia │ FishAudio                              │  │
│  └───────────────────────────────────────────────────────────┘  │
│  ┌─────────────── video/ ─────────────────────────────────────┐  │
│  │ Sora │ Kling │ Wan │ Grok │ Runway │ Seedance │ Hailuo    │  │
│  │ Pika │ Luma │ OmniHuman │ HappyHorse                       │  │
│  └───────────────────────────────────────────────────────────┘  │
│  Plan() → optimal upstream format (score-based)                  │
│  SSE stream converters: Claude↔OpenAI (bidirectional)            │
└──────────────────────────────────────────────────────────────────┘
```

### Three-Layer Design

**model/** — Provider types, channel config
- 40+ `ProviderType` (OpenAI=1 ... Midjourney=40)
- `Channel` struct with protocol list + API key
- `ResolveProtocol()` maps provider → default protocol

**translator/** — Format conversion (Text)
- `Convert(body, from, to)` — unified entry point
- `DetectFormat(body, path)` — path first, body heuristics
- 4 type definition files + 12 converter functions
- Extensible: add a file + `convertDirect` case

**executor/** — Modality-specific provider execution
- Text: `executor/text` — `Executor` interface: Init, NativeEndpoints, Convert, Customize, Stream, DoRequest
  - `Register("name", &Executor{})` — plugin registry via init()
  - `Plan(in, out, endpoints)` — upstream format selection (score: input+output mismatch)
  - `RequestInfo.UpstreamFormat` — zero-value triggers Plan; 4-level override
- Image: `executor/image` — `ImageExecutor` interface (TextToImage, ImageToImage, GetTask)
  - Sync GPT Image, async polling for Midjourney/Qwen/Wan/Seedream
- Audio: `executor/audio` — `AudioExecutor` interface (TextToSpeech/*AudioStream*, SpeechToText, MusicGenerate, GetTask, ListVoices)
  - TTS returns `*AudioStream` — one chunk sync vs multi-chunk streaming
- Video: `executor/video` — `VideoExecutor` interface (TextToVideo...+GetTask)
  - All video is async polling

---

## Provider Implementations — Text

| Executor | Native Formats | Streaming | Tests |
|---|---|---|---|
| **Claude** | `claude` (`/v1/messages`) | ✅ Claude↔OpenAI | translator-level |
| **OpenAI** | `openai` (`/v1/chat/completions`) | ✅ Native | translator-level |
| **Gemini** | `gemini` (Google endpoint) | ⚠️ Via OpenAI hub | translator-level |
| **DeepSeek** | `openai` + `claude` dual | ✅ Bidirectional | **27 tests** |
| **Volcengine** | `openai` + `openai_responses` dual | ✅ Native SSE | **32 tests** |

### DeepSeek
- Dual endpoints: OpenAI `/v1/chat/completions` + Claude `/anthropic/v1/messages`
- Auth: Bearer (OpenAI) / `x-api-key` (Claude)
- Thinking/reasoning injection with effort mapping (minimal→max)
- 27 tests: Chat, streaming, conversion, Plan, tools, thinking, errors

### Volcengine / Doubao (火山引擎)
- OpenAI Chat + Responses endpoints
- Auth: `Authorization: Bearer` + model-in-body
- Multi-model: doubao-seed-2-0-lite, GLM-4-7B, DeepSeek V3
- Bot model routing (`bot-` prefix → `/api/v3/bots/chat/completions`)
- `stream_options: {"include_usage": true}` injection
- 32 tests: 3-model Chat, Responses, streaming, 10-way conversion, Plan, tools, params

---

## Provider Implementations — Image

| Executor | T2I | I2I | Pattern | Auth | Notes |
|----------|-----|-----|---------|------|-------|
| **GPT Image** | ✅ | ✅(edits) | Sync | Bearer | OpenAI DALL-E / GPT Image 2 |
| **Qwen Image** | ✅ | ✅ | Async | Bearer | DashScope qwen-max/plus/turbo |
| **NanoBanana** | ✅ | ❌ | Sync | Bearer | OpenAI-compatible |
| **Z Image** | ✅ | ❌ | Sync | Bearer | OpenAI-compatible |
| **Wan** | ✅ | ✅ | Async | Bearer | DashScope wan2.5-t2i/i2i |
| **Seedream** | ✅ | ✅ | Sync | Key | fal.ai, 5.0/4.5/4.0, sync |
| **Midjourney** | ✅ | ✅ | Async | Bearer | /v1/imagine → poll, I2I via img URL in prompt |

> **Models, endpoints, auth, and Extra params per provider:** [docs/provider-reference.md](docs/provider-reference.md)

---

## Provider Implementations — Audio

| Executor | TTS | STT | Music | Pattern | Notes |
|----------|-----|-----|-------|---------|-------|
| **OpenAI** | ✅ | ✅ | ❌ | Sync | /v1/audio/speech + transcriptions |
| **ElevenLabs** | ✅ | ❌ | ❌ | Sync | POST /v1/text-to-speech/{voice_id} |
| **CosyVoice** | ✅ | ❌ | ❌ | Sync/URL | DashScope SpeechSynthesizer |
| **Suno** | ❌ | ❌ | ✅ | Async | Music gen via relay |
| **FunASR** | ❌ | ✅ | ❌ | Sync+Async | DashScope + self-hosted |
| **Azure** | ✅ | ✅ | ❌ | Sync | SSML-TTS + REST STT |
| **PlayHT** | ✅ | ❌ | ❌ | Sync | /v2/tts/stream |
| **Cartesia** | ✅ | ❌ | ❌ | Sync | Sonic-3 ultra-low-latency |
| **Fish Audio** | ✅ | ❌ | ❌ | Sync | Zero-shot voice clone |

> **Models, endpoints, auth, and Extra params per provider:** [docs/provider-reference.md](docs/provider-reference.md)

### TTS Streaming

`TextToSpeech` returns `*AudioStream` — unified sync/streaming interface:

```go
stream, _ := tts.TextToSpeech(req)

// Streaming: iterate chunks as they arrive
for chunk := range stream.Chunk {
    audioSink.Write(chunk.Data)
}

// Or sync convenience: drain to single result
result, _ := stream.Collect() // *AudioResult with full audio bytes
```

Sync providers wrap result with `audio.NewStreamFromResult()`. Streaming providers push chunks to channel. Caller decides pattern.

---

## Provider Implementations — Video

All video providers are **async** — return pending task, poll via `GetTask`:

| Executor | T2V | I2V | V2V | Extend | Edit | Notes |
|----------|-----|-----|-----|--------|------|-------|
| **Sora** | ✅ | ✅ | ❌ | ❌ | ❌ | OpenAI (deprecating Sep 24, 2026) |
| **Kling** | ✅ | ✅ | ❌ | ❌ | ❌ | Kuaishou, JWT auth |
| **Wan** | ✅ | ✅ | ❌ | ❌ | ❌ | DashScope wan2.7-t2v/i2v |
| **Grok** | ✅ | ✅ | ❌ | ❌ | ❌ | xAI, cheapest |
| **Runway** | ✅ | ✅ | ❌ | ❌ | ❌ | Gen-4, X-Runway-Version |
| **Seedance** | ✅ | ❌ | ❌ | ❌ | ❌ | ByteDance via fal.ai, 2K |
| **Hailuo** | ✅ | ✅ | ❌ | ❌ | ❌ | MiniMax |
| **Pika** | ✅ | ✅ | ❌ | ❌ | ❌ | fal.ai, pikaffects |
| **Luma** | ✅ | ✅ | ❌ | ❌ | ❌ | Ray3.2 via fal.ai |
| **OmniHuman** | ❌ | ✅ | ❌ | ❌ | ❌ | Bytedance avatar (img+audio→video) |
| **HappyHorse** | ✅ | ✅ | ❌ | ❌ | ❌ | DashScope, same infra as Wan |

> **Models, endpoints, auth, and Extra params per provider:** [docs/provider-reference.md](docs/provider-reference.md)

---

## Test Coverage — 96 Tests, All Passing ✅

```
Package                    Tests     Notes
─────────────────────────────────────────────────
translator/                  37      No API keys needed
executor/text/deepseek/      27      Needs DEEPSEEK_API_KEY
executor/text/volcengine/    32      Needs VOLC_API_KEY
─────────────────────────────────────────────────
Total                        96      go test ./... -count=1 -timeout 300s
```

```bash
go test ./translator/                             # 37 unit tests
go test ./executor/text/deepseek/ -timeout 120s    # 27 integration
go test ./executor/text/volcengine/ -timeout 180s  # 32 integration
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
├── model/
│   └── model.go              # ProviderType (40+), Channel, ResolveProtocol
├── translator/
│   ├── translator.go         # Convert(), DetectFormat(), Format constants
│   ├── conv.go               # 12 directional converters
│   ├── conv_test.go          # 37 tests
│   ├── openai.go             # OpenAI Chat type defs
│   ├── claude.go             # Claude Messages type defs
│   ├── gemini.go             # Gemini type defs
│   └── responses.go          # Responses API type defs
├── executor/
│   ├── text/
│   │   ├── executor.go        # Executor interface, RequestInfo, Plan()
│   │   ├── registry.go        # Plugin registry
│   │   ├── shared.go          # Helpers (ReplaceModelField, etc.)
│   │   ├── stream_exec.go     # Stream execution pipeline
│   │   ├── streams.go         # SSE converters (Claude↔OpenAI)
│   │   ├── claude/            # Claude executor
│   │   ├── openai/            # OpenAI executor
│   │   ├── gemini/            # Gemini executor
│   │   ├── deepseek/          # DeepSeek (27 tests)
│   │   └── volcengine/        # Volcengine/Doubao (32 tests)
│   ├── image/                 # 7 image providers (GPT Image, Midjourney, etc.)
│   ├── audio/                 # 9 audio providers (TTS/STT/Music)
│   └── video/                 # 11 video providers
├── docs/
│   ├── image-generation.md    # Image integration spec
│   ├── audio-speech.md        # Audio/speech integration spec
│   ├── video-generation.md    # Video integration spec
│   └── provider-reference.md  # Per-provider models/endpoints/params
├── CLAUDE.md                  # Dev conventions
├── go.mod                     # Go 1.23, zero external deps
└── README.md
```

---

## Adding a New Provider

### Text Chat Provider
1. **Define `ProviderType`** in `model/model.go`
2. **Add format types** (if new protocol) in `translator/`
3. **Implement `text.Executor`** in `executor/text/<name>.go` with `init()` registration
4. **Define `NativeEndpoints()`** — supported formats + URL paths
5. **Add vendor logic** in `RequestCustomize`/`ResponseCustomize`
6. **Write tests** — unit + integration in `executor/text/<name>/<name>_test.go`

### Image / Audio / Video Provider
1. **Choose modality**: `executor/image/`, `executor/audio/`, or `executor/video/`
2. **Implement executor interface** (e.g. `ImageExecutor`, `AudioExecutor`, `VideoExecutor`)
3. **Register** via `RegisterImage()`, `RegisterAudio()`, `RegisterVideo()` in `init()`
4. **TTS note**: sync → wrap with `audio.NewStreamFromResult()`. For streaming → push to `AudioStream.Chunk`
5. **Video note**: all video is async — return `*VideoTask` with status `pending`, implement `GetTask` for polling
6. **Write tests** in modality-specific directory

---

---

## License

MIT
