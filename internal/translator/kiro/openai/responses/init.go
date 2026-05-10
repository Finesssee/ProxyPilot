package responses

import (
	. "github.com/router-for-me/CLIProxyAPI/v7/internal/constant"
	"github.com/router-for-me/CLIProxyAPI/v7/internal/interfaces"
	"github.com/router-for-me/CLIProxyAPI/v7/sdk/translator"
)

func init() {
	// Register OpenaiResponse -> Kiro translation for /v1/responses endpoint
	translator.Register(
		OpenaiResponse,
		Kiro,
		ConvertOpenAIResponsesRequestToKiro,
		interfaces.TranslateResponse{
			Stream:    ConvertKiroResponseToOpenAIResponses,
			NonStream: ConvertKiroResponseToOpenAIResponsesNonStream,
		},
	)
}
