// Package model defines shared data types for channel configuration and provider metadata.
package model

// ProviderType maps to llm_channels.provider_type.
type ProviderType int

const (
	ProviderOpenAI     ProviderType = 1
	ProviderClaude     ProviderType = 2
	ProviderGemini     ProviderType = 3
	ProviderAli        ProviderType = 4
	ProviderTencent    ProviderType = 6
	ProviderZhipu      ProviderType = 7
	ProviderDeepSeek   ProviderType = 8
	ProviderMoonshot   ProviderType = 9
	ProviderVolcengine ProviderType = 10
	ProviderAWS        ProviderType = 11
	ProviderAzure      ProviderType = 12
	ProviderVertex     ProviderType = 13
	ProviderMistral    ProviderType = 15
	ProviderXAI        ProviderType = 16
	ProviderAI360      ProviderType = 17
	ProviderLingyi     ProviderType = 18
	ProviderBaiduV2    ProviderType = 19
	ProviderCloudflare ProviderType = 20
	ProviderOllama     ProviderType = 22
	ProviderSiliconFlow ProviderType = 25
	ProviderXunfei     ProviderType = 26
	ProviderOpenRouter ProviderType = 27
	ProviderXInference ProviderType = 28
	ProviderMiniMax    ProviderType = 29
	ProviderSubmodel   ProviderType = 30
	ProviderCoze       ProviderType = 32
	ProviderDify       ProviderType = 33
	ProviderJimeng     ProviderType = 34
	ProviderCodex      ProviderType = 35
	ProviderSora       ProviderType = 37
	ProviderKling      ProviderType = 38
	ProviderSuno       ProviderType = 39
	ProviderMidjourney ProviderType = 40
)

// ProtocolType identifies the upstream API protocol format.
type ProtocolType string

const (
	ProtocolOpenAI    ProtocolType = "openai-compatible"
	ProtocolClaude    ProtocolType = "anthropic-compatible"
	ProtocolGemini    ProtocolType = "gemini-compatible"
	ProtocolAli       ProtocolType = "ali-compatible"
	ProtocolBaidu     ProtocolType = "baidu-compatible"
	ProtocolZhipu     ProtocolType = "zhipu-compatible"
	ProtocolVolcengine ProtocolType = "volcengine-compatible"
	ProtocolAWS       ProtocolType = "bedrock"
)

// ResolveProtocol maps ProviderType to ProtocolType.
func ResolveProtocol(pt ProviderType) ProtocolType {
	switch pt {
	case ProviderClaude:
		return ProtocolClaude
	case ProviderGemini, ProviderVertex:
		return ProtocolGemini
	case ProviderAli:
		return ProtocolAli
	case ProviderBaiduV2:
		return ProtocolBaidu
	case ProviderZhipu:
		return ProtocolZhipu
	case ProviderVolcengine:
		return ProtocolVolcengine
	case ProviderAWS:
		return ProtocolAWS
	default:
		return ProtocolOpenAI
	}
}

// ProtocolEntry represents one protocol + base_url pair for a channel.
type ProtocolEntry struct {
	Protocol          ProtocolType `json:"protocol"`
	BaseURL           string       `json:"base_url"`
	UpstreamModelName string       `json:"upstream_model_name"`
	IsModelMapped     bool         `json:"is_model_mapped"`
}

// Channel maps to llm_channels table.
type Channel struct {
	ID              int64           `json:"id"`
	Code            string          `json:"code"`
	Name            string          `json:"name"`
	Description     string          `json:"description"`
	ProviderType    ProviderType    `json:"provider_type"`
	Protocols       []ProtocolEntry `json:"protocols"`
	ApiKeyEncrypted string          `json:"-"`
	Settings        ChannelSettings `json:"settings"`
	Status          string          `json:"status"`
	CreatedAt       int64           `json:"created_at"`
	UpdatedAt       int64           `json:"updated_at"`
}

// ChannelSettings represents channel-level configuration.
type ChannelSettings struct {
	TimeoutSeconds int `json:"timeout_seconds,omitempty"`
	MaxRetries     int `json:"max_retries,omitempty"`
}

// ResolveProtocol picks the ProtocolEntry matching the given protocol type.
func (c *Channel) ResolveProtocol(proto ProtocolType) *ProtocolEntry {
	for i := range c.Protocols {
		if c.Protocols[i].Protocol == proto {
			return &c.Protocols[i]
		}
	}
	// Fallback: first entry.
	if len(c.Protocols) > 0 {
		return &c.Protocols[0]
	}
	return nil
}
