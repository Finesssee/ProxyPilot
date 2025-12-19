package embeddings

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"
)

type OllamaClient struct {
	BaseURL string
	Model   string
	Client  *http.Client
}

type ollamaEmbedRequest struct {
	Model string         `json:"model"`
	Input []string       `json:"input"`
	Opts  *ollamaOptions `json:"options,omitempty"`
}

type ollamaOptions struct {
	Truncate bool `json:"truncate,omitempty"`
}

type ollamaEmbedResponse struct {
	Embeddings [][]float64 `json:"embeddings"`
}

func (c *OllamaClient) Embed(ctx context.Context, inputs []string) ([][]float32, error) {
	if c == nil {
		return nil, errors.New("ollama client is nil")
	}
	if len(inputs) == 0 {
		return nil, nil
	}
	base := strings.TrimSpace(c.BaseURL)
	if base == "" {
		base = "http://127.0.0.1:11434"
	}
	model := strings.TrimSpace(c.Model)
	if model == "" {
		model = "embeddinggemma"
	}
	httpClient := c.Client
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 10 * time.Second}
	}

	reqBody, err := json.Marshal(ollamaEmbedRequest{
		Model: model,
		Input: inputs,
		Opts:  &ollamaOptions{Truncate: true},
	})
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, base+"/api/embed", bytes.NewReader(reqBody))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, errors.New("ollama embed: non-2xx response")
	}

	var out ollamaEmbedResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	if len(out.Embeddings) == 0 {
		return nil, nil
	}

	vecs := make([][]float32, 0, len(out.Embeddings))
	for _, v := range out.Embeddings {
		if len(v) == 0 {
			vecs = append(vecs, nil)
			continue
		}
		f := make([]float32, len(v))
		for i := range v {
			f[i] = float32(v[i])
		}
		vecs = append(vecs, f)
	}
	return vecs, nil
}
