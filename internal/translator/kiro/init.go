// Package kiro provides translation functionality for Kiro (Amazon Q) API compatibility.
// It registers translators for converting between OpenAI, Claude, and Kiro API formats,
// handling the conversion of requests and responses across different AI service formats.
package kiro

import (
	_ "github.com/router-for-me/CLIProxyAPI/v6/internal/translator/kiro/claude"
	_ "github.com/router-for-me/CLIProxyAPI/v6/internal/translator/kiro/openai/chat-completions"
	_ "github.com/router-for-me/CLIProxyAPI/v6/internal/translator/kiro/openai/responses"
)
