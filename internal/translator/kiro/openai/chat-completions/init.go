package chat_completions

import (
	. "github.com/router-for-me/CLIProxyAPI/v6/internal/constant"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/interfaces"
	"github.com/router-for-me/CLIProxyAPI/v6/sdk/translator"
)

func init() {
	// Register OpenAI -> Kiro translation for /v1/chat/completions
	translator.Register(
		OpenAI,
		Kiro,
		ConvertOpenAIRequestToKiro,
		interfaces.TranslateResponse{
			Stream:     ConvertKiroResponseToOpenAI,
			NonStream:  ConvertKiroResponseToOpenAINonStream,
			TokenCount: OpenAITokenCount,
		},
	)
}
