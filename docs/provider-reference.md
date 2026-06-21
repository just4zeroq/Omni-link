# Provider Reference

Detailed models, endpoints, parameters, and auth for all integrated providers.

---

## Image Providers

### GPT Image (OpenAI)

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `model` | string | — | `dall-e-3`, `gpt-image-2`, `gpt-image-2-pro` |
| `prompt` | string | required | Text description |
| `n` | int | 1 | Images to generate |
| `size` | string | — | `1024x1024`, `1792x1024`, etc. |
| `quality` | string | — | `standard`, `hd` |
| `response_format` | string | `url` | `url` or `b64_json` |

**Extra params:**
| Key | Type | Description |
|-----|------|-------------|
| `style` | string | `vivid` or `natural` (OpenAI-specific) |

**Endpoints:** `POST /v1/images/generations` (T2I), `POST /v1/images/edits` (I2I)
**Auth:** `Authorization: Bearer`
**Pattern:** Sync

---

### Qwen Image (Alibaba DashScope)

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `model` | string | `qwen-max` | `qwen-max`, `qwen-plus`, `qwen-turbo` |
| `prompt` | string | required | Text description |
| `size` | string | — | Image dimensions |
| `n` | int | 1 | Image count |
| `quality` | string | — | Quality level |
| `response_format` | string | — | `b64_json` |

**I2I extra params:**
| Key | Type | Description |
|-----|------|-------------|
| `image` | string | Source image URL or base64 |
| `strength` | float64 | 0-1 transformation degree |

**Endpoint:** `POST /api/v1/services/aigc/text2image/image-synthesis`
**Query:** `GET /api/v1/tasks/{id}`
**Auth:** `Authorization: Bearer`
**Pattern:** Async (DashScope polling)

---

### NanoBanana

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `model` | string | — | `nanobanana-2`, `nanobanana-pro` |
| `prompt` | string | required | Text description |
| `n` | int | 1 | Images to generate |
| `size` | string | — | Image dimensions |
| `quality` | string | — | Quality level |
| `response_format` | string | `url` | `url` or `b64_json` |

Extra params passthrough (reserved: `model`, `prompt`, `n`, `size`, `quality`, `response_format`).

**Endpoint:** `POST /v1/images/generations` (OpenAI-compatible)
**Auth:** `Authorization: Bearer`
**Pattern:** Sync
**I2I:** Not supported

---

### Z Image Turbo

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `model` | string | — | `z-image-turbo` |
| `prompt` | string | required | Text description |
| `n` | int | 1 | Images to generate |
| `size` | string | — | Image dimensions |
| `quality` | string | — | Quality level |
| `response_format` | string | `url` | `url` or `b64_json` |

All Extra passthrough to body.

**Endpoint:** `POST /v1/images/generations` (OpenAI-compatible)
**Auth:** `Authorization: Bearer`
**Pattern:** Sync
**I2I:** Not supported

---

### Wan2.5 Image (Alibaba DashScope)

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `model` | string | `wan2.5-t2i` | `wan2.5-t2i`, `wan2.5-i2i` |
| `prompt` | string | required | Text description |
| `size` | string | — | Image dimensions |
| `quality` | string | — | Quality level |

**I2I extra params:**
| Key | Type | Description |
|-----|------|-------------|
| `image` | string | Source image URL |
| `mask` | string | Inpainting mask |
| `strength` | float64 | 0-1 transformation degree |

**Endpoint:** `POST /api/v1/services/aigc/text2image/image-synthesis`
**Query:** `GET /api/v1/tasks/{id}`
**Auth:** `Authorization: Bearer`
**Pattern:** Async (DashScope polling)

---

### Seedream (ByteDance via fal.ai)

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `model` | string | `seedream-4.5` | `seedream-5.0`, `seedream-4.5`, `seedream-4.0` |
| `prompt` | string | required | Text description |
| `size` | string | — | Image size (mapped to `image_size`) |
| `n` | int | 1 | Image count (mapped to `num_images`) |

**I2I extra params:**
| Key | Type | Description |
|-----|------|-------------|
| `image_url` | string | Source image URL |
| `strength` | float64 | 0-1 transformation degree |

**Endpoints:**
- T2I: `POST /fal-ai/seedream/{version}/text-to-image`
- I2I: `POST /fal-ai/seedream/{version}/image-to-image`
**Auth:** `Authorization: Key`
**Pattern:** Sync (fal.ai returns immediately)

---

### Midjourney

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `model` | string | — | Midjourney model version |
| `prompt` | string | required | Text + optional image URL |
| `ratio` | string | `1:1` | Aspect ratio (via Extra) |

**Extra params:**
| Key | Type | Description |
|-----|------|-------------|
| `ratio` | string | Aspect ratio e.g. `16:9`, `1:1` (default) |
| `style` | string | Style preset |
| `no` | string | Negative prompt |
| `chaos` | float64 | Chaos level (0-100) |

**Endpoints:**
- Generate: `POST /v1/imagine`
- Query: `GET /v1/task/{id}/fetch`
**Auth:** `Authorization: Bearer`
**Pattern:** Async (poll GetTask)

---

## Audio Providers

### OpenAI TTS/STT

**TTS fields:**
| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `model` | string | `tts-1` | `gpt-4o-mini-tts`, `tts-1`, `tts-1-hd` |
| `input` | string | required | Text to speak |
| `voice` | string | `coral` | `alloy`, `echo`, `coral`, etc. |
| `instructions` | string | — | Voice instructions (gpt-4o-mini-tts) |
| `response_format` | string | `mp3` | `mp3`, `opus`, `aac`, `flac`, `pcm` |
| `speed` | float64 | 1.0 | 0.25-4.0 |

**STT fields:**
| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `model` | string | `whisper-1` | `whisper-1`, `gpt-4o-transcribe`, `gpt-4o-mini-transcribe` |
| `file` | []byte | required | Audio file content |
| `file_name` | string | required | `audio.mp3`, etc. |
| `language` | string | — | ISO-639-1 |
| `prompt` | string | — | Context prompt |
| `response_format` | string | — | `json`, `verbose_json`, `srt`, `vtt` |
| `temperature` | float64 | — | 0-1 |

**Endpoints:** `POST /v1/audio/speech` (TTS), `POST /v1/audio/transcriptions` (STT)
**Auth:** `Authorization: Bearer`
**Pattern:** Sync

---

### ElevenLabs TTS

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `model` | string | `eleven_turbo_v2` | `eleven_v3`, `eleven_flash_v2`, `eleven_turbo_v2` |
| `input` | string | required | Text to speak |
| `voice` | string | Rachel | Voice ID |

**Extra params:**
| Key | Type | Description |
|-----|------|-------------|
| `stability` | float64 | Voice stability (0-1) |
| `similarity_boost` | float64 | Similarity boost (0-1) |

**Endpoints:** `POST /v1/text-to-speech/{voice_id}` (TTS), `GET /v1/voices` (list)
**Auth:** `xi-api-key`
**Pattern:** Sync
**STT/Music:** Not supported

---

### CosyVoice (Alibaba DashScope)

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `model` | string | `cosyvoice-v3.5-flash` | `cosyvoice-v3.5-plus`, `cosyvoice-v3.5-flash` |
| `input` | string | required | Text to speak |
| `voice` | string | — | Voice ID or name |
| `response_format` | string | — | Audio format |
| `speed` | float64 | — | Speed multiplier |

**Endpoint:** `POST /api/v1/services/audio/tts/SpeechSynthesizer`
**Auth:** `Authorization: Bearer`
**Pattern:** Sync/URL (direct audio or URL response)
**STT/Music/ListVoices:** Not supported

---

### Suno Music Generation

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `model` | string | `suno-v5` | `suno-v5`, `chirp-v5`, `suno-v4`, `chirp-v4` |
| `prompt` | string | required | Song description or lyrics |
| `title` | string | — | Song title |
| `tags` | string | — | Genre/style tags |
| `instrumental` | bool | false | Instrumental only |
| `duration` | int | — | Desired duration (seconds) |
| `callback_url` | string | — | Webhook URL |

**Endpoints:**
- Generate: `POST /v1/music/generate`
- Query: `GET /v1/music/{id}`
**Auth:** `Authorization: Bearer`
**Pattern:** Async (poll GetTask)
**TTS/STT:** Not supported

---

### FunASR STT

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `model` | string | `fun-asr` | STT model |
| `file` | []byte | required | Audio file content |
| `file_name` | string | required | Audio filename |
| `language` | string | — | Language |

**Cloud mode (DashScope):**
| Extra Key | Type | Description |
|-----------|------|-------------|
| `audio_url` | string | Public audio URL (alternative to file upload) |

**Self-hosted mode (OpenAI-compatible):** Uses `POST /v1/audio/transcriptions` with multipart upload.

**Endpoints:**
- Cloud: `POST /api/v1/services/audio/asr/transcription`
- Cloud query: `GET /api/v1/tasks/{id}`
- Self-hosted: `POST {baseURL}/v1/audio/transcriptions`
**Auth:** `Authorization: Bearer`
**Pattern:** Cloud=Async polling, Self-hosted=Sync
**TTS/Music:** Not supported

---

### Azure Speech Services

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `input` | string | required | Text to speak (TTS) |
| `voice` | string | `zh-CN-XiaoxiaoNeural` | Azure voice name |
| `response_format` | string | `mp3` | `mp3`, `wav`, `opus`, `pcm` |

**Voice short names:** `xiaoxiao`, `xiaoyi`, `yunxi`, `yunye`, `jenny`, `guy`, `aria`, `davis`

**Extra params:**
| Key | Type | Description |
|-----|------|-------------|
| `lang` | string | Language (default `zh-CN`) |

**STT fields:**
| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `file` | []byte | required | Audio content |
| `file_name` | string | — | Determines Content-Type |
| `language` | string | `zh-CN` | Recognition language |

**Endpoints:**
- TTS: `https://{region}.tts.speech.microsoft.com/cognitiveservices/v1`
- STT: `https://{region}.stt.speech.microsoft.com/speech/recognition/conversation/cognitiveservices/v1`
**Auth:** `Ocp-Apim-Subscription-Key`
**Pattern:** Sync
**Music:** Not supported
**ListVoices:** Not supported (use Azure Portal)

---

### PlayHT TTS

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `model` | string | `PlayHT2.0-turbo` | `PlayHT2.0`, `Play3.0-mini`, `PlayDialog` |
| `input` | string | required | Text to speak |
| `voice` | string | (default Aditi) | Voice URI |
| `speed` | float64 | — | Speed multiplier |
| `response_format` | string | `mp3` | Output format |

**Endpoints:** `POST /v2/tts/stream` (TTS), `GET /v2/voices` (list)
**Auth:** `Authorization: Bearer` + `X-User-ID`
**Pattern:** Sync
**STT/Music:** Not supported

---

### Cartesia TTS

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `model` | string | `sonic-3` | Sonic-3 |
| `input` | string | required | Text to speak |
| `voice` | string | (Sonic default) | Voice UUID |
| `response_format` | string | `mp3` | Output format |

**Extra params:**
| Key | Type | Description |
|-----|------|-------------|
| `speed` | string | Speed e.g. `slow`, `normal`, `fast` |
| `emotion` | string | Emotion e.g. `anger`, `cheerfulness` |
| `language` | string | Language |

**Endpoints:** `POST /tts/bytes` (TTS), `GET /voices` (list)
**Auth:** `Authorization: Bearer`
**Pattern:** Sync (~90ms first-byte latency)
**STT/Music:** Not supported

---

### Fish Audio TTS

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `model` | string | — | `fish-speech` variants |
| `input` | string | required | Text to speak |
| `voice` | string | — | Voice ID |
| `response_format` | string | `mp3` | `mp3`, `wav`, etc. |
| `speed` | float64 | — | Speed multiplier |

**Extra params:**
| Key | Type | Description |
|-----|------|-------------|
| `language` | string | Language |

**Endpoints:** `POST /v1/tts` (TTS), `GET /v1/voices` (list)
**Auth:** `Authorization: Bearer`
**Pattern:** Sync (audio stream or URL)
**STT/Music:** Not supported

---

## Video Providers

All video providers are **async** — `TextToVideo`/`ImageToVideo` return pending `VideoTask`, poll via `GetTask`.

### Sora (OpenAI)

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `model` | string | `sora-2` | `sora-2`, `sora-2-pro` |
| `prompt` | string | required | Text description |
| `duration` | int | — | Video duration (seconds) |
| `size` | string | — | Resolution |
| `quality` | string | — | `standard`, `pro` |

⚠️ OpenAI discontinuing Sora 2 on September 24, 2026.

**I2V:** Yes (via `image` field in request body)
**Endpoints:** `POST /v1/videos` (create), `GET /v1/videos/{id}` (poll)
**Auth:** `Authorization: Bearer`
**V2V/Extend/Edit/CreateCharacter:** Not supported

---

### Kling (Kuaishou)

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `model` | string | `kling-v3` | `kling-v3`, `kling-v2.6`, `kling-video-o1` |
| `prompt` | string | required | Text description |
| `image` | string | — | Source image URL (I2V) |
| `size` | string | — | Resolution |
| `duration` | int | — | Duration (seconds) |

**Endpoints:**
- T2V: `POST /v1/videos/text2video`
- I2V: `POST /v1/videos/image2video`
- Query: `GET /v1/videos/text2video/{id}`
**Auth:** JWT (AK/SK signed, 30-min expiry) via `Authorization: Bearer`
**V2V/Extend/Edit/CreateCharacter:** Not supported

---

### Wan Video (Alibaba DashScope)

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `model` | string | — | `wan2.7-t2v`, `wan2.7-i2v` |
| `prompt` | string | required | Text description |
| `image` | string | — | Source image URL (I2V) |
| `size` | string | — | Resolution |
| `duration` | int | — | Duration (seconds) |

**Endpoint:** `POST /api/v1/services/aigc/video-generation/video-synthesis`
**Auth:** `Authorization: Bearer`
**V2V/Extend/Edit/CreateCharacter:** Not supported

---

### Grok (xAI)

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `model` | string | `grok-imagine-video-1.5` | `grok-imagine-video-1.5`, `grok-imagine-video-1.5-preview` |
| `prompt` | string | required | Text description |
| `image` | string | — | Source image URL (I2V) |
| `size` | string | — | Resolution |

**Auth:** `Authorization: Bearer`
**V2V/Extend/Edit/CreateCharacter:** Not supported

---

### Runway Gen-4

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `model` | string | `gen4_turbo` | `gen4.5`, `gen4_turbo`, `gen4_aleph` |
| `prompt` | string | required | Text description |
| `prompt_image` | string | — | Source image URL (I2V) |
| `duration` | int | — | Duration (seconds) |
| `resolution` | string | — | Video resolution (mapped from `Size`) |

**Endpoints:**
- T2V: `POST /v1/text_to_video`
- I2V: `POST /v1/image_to_video`
- Query: `GET /v1/tasks/{id}`
**Auth:** `Authorization: Bearer` + `X-Runway-Version: 2025-03-13`
**V2V/Extend/Edit/CreateCharacter:** Not supported

---

### Seedance (ByteDance via fal.ai)

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `prompt` | string | required | Text description |
| `size` | string | — | Resolution (up to 2K) |

**Endpoint:** `POST /fal-ai/bytedance/seedance-2.0/text-to-video`
**Auth:** `Authorization: Key`
**I2V/V2V/Extend/Edit/CreateCharacter:** Not supported

---

### Hailuo (MiniMax)

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `model` | string | `MiniMax-Hailuo-2.3` | `MiniMax-Hailuo-2.3`, `MiniMax-Hailuo-02` |
| `prompt` | string | required | Text description |
| `image` | string | — | Source image URL (I2V) |

**Auth:** `Authorization: Bearer`
**V2V/Extend/Edit/CreateCharacter:** Not supported

---

### Pika (via fal.ai)

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `model` | string | `pika-v2.2` | Pika version |
| `prompt` | string | required | Text description |
| `image_url` | string | — | Source image URL (I2V) |
| `aspect_ratio` | string | — | Mapped from `Size` |

Features: Pikaffects, Pikascenes (via Extra passthrough).

**Endpoints:**
- T2V: `POST /fal-ai/pika/v2.2/text-to-video`
- I2V: `POST /fal-ai/pika/v2.2/image-to-video`
- Query: `GET /fal-ai/pika/v2.2/requests/{id}/status`
**Auth:** `Authorization: Key`
**V2V/Extend/Edit/CreateCharacter:** Not supported (despite Pika API supporting them natively — not yet integrated)

---

### Luma Ray3.2 (via fal.ai)

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `model` | string | — | `ray-3.2`, `ray-2`, `ray-flash-2` |
| `prompt` | string | required | Text description |
| `image` | string | — | Source image URL (I2V) |

**Auth:** `Authorization: Key`
**V2V/Extend/Edit/CreateCharacter:** Not supported

---

### OmniHuman (ByteDance via fal.ai)

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `image` | string | required | Reference image (avatar) |
| `prompt` | string | — | Audio description — NOT general T2V |

**Note:** OmniHuman is avatar-only — takes a person image + audio (in Extra), generates talking-head video.

**Extra params:**
| Key | Type | Description |
|-----|------|-------------|
| `audio_url` | string | Audio source for avatar |

**Auth:** `Authorization: Key`
**T2V/V2V/Extend/Edit/CreateCharacter:** Not supported

---

### HappyHorse (Alibaba DashScope)

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `model` | string | — | `happyhorse-1.0-t2v`, `happyhorse-1.0-i2v` |
| `prompt` | string | required | Text description |
| `image` | string | — | Source image URL (I2V) |

Same DashScope infrastructure as Wan.

**Endpoint:** `POST /api/v1/services/aigc/video-generation/video-synthesis`
**Auth:** `Authorization: Bearer`
**V2V/Extend/Edit/CreateCharacter:** Not supported
