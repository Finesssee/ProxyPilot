package executor

import (
	"github.com/router-for-me/CLIProxyAPI/v6/internal/config"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/runtime/executor/helps"
	cliproxyexecutor "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/executor"
)

func applyPayloadConfig(cfg *config.Config, model string, payload []byte) []byte {
	return helps.ApplyPayloadConfigWithRoot(cfg, model, "", "", payload, nil, "", "")
}

func applyPayloadConfigWithRoot(cfg *config.Config, model, protocol, root string, payload, original []byte, requestedModel string) []byte {
	return helps.ApplyPayloadConfigWithRoot(cfg, model, protocol, root, payload, original, requestedModel, "")
}

func payloadRequestedModel(opts cliproxyexecutor.Options, fallback string) string {
	return helps.PayloadRequestedModel(opts, fallback)
}

func ApplyReasoningEffortMetadata(payload []byte, metadata map[string]any, model string, path string, force bool) []byte {
	return helps.ApplyReasoningEffortMetadata(payload, metadata, model, path, force)
}

func EstimateRequestTokens(model string, body []byte) (int64, error) {
	return helps.EstimateRequestTokens(model, body)
}
