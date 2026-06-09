package memory

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"io"
	"net/http"
	"time"

	"CompeteAI/settings"
)

const hashEmbedDim = 128

// HashEmbedder 确定性本地 embedder（无外部 API 依赖）。
type HashEmbedder struct{ dim int }

func NewHashEmbedder(dim int) Embedder {
	if dim <= 0 {
		dim = hashEmbedDim
	}
	return &HashEmbedder{dim: dim}
}

func (e *HashEmbedder) Dim() int { return e.dim }

func (e *HashEmbedder) Embed(_ context.Context, texts []string) ([][]float32, error) {
	out := make([][]float32, len(texts))
	for i, t := range texts {
		out[i] = e.hashEmbed(t)
	}
	return out, nil
}

func (e *HashEmbedder) hashEmbed(text string) []float32 {
	vec := make([]float32, e.dim)
	for _, word := range tokenize(text) {
		h := fnv.New32a()
		_, _ = h.Write([]byte(word))
		idx := int(h.Sum32()) % e.dim
		vec[idx] += 1
	}
	var norm float64
	for _, v := range vec {
		norm += float64(v * v)
	}
	if norm > 0 {
		n := float32(1.0 / (norm + 1e-9))
		for i := range vec {
			vec[i] *= n
		}
	}
	return vec
}

func tokenize(s string) []string {
	s = stringsToLower(s)
	var words []string
	var cur []rune
	flush := func() {
		if len(cur) > 1 {
			words = append(words, string(cur))
		}
		cur = cur[:0]
	}
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r > 127 {
			cur = append(cur, r)
		} else {
			flush()
		}
	}
	flush()
	return words
}

func stringsToLower(s string) string {
	b := []byte(s)
	for i := range b {
		if b[i] >= 'A' && b[i] <= 'Z' {
			b[i] += 'a' - 'A'
		}
	}
	return string(b)
}

// ---------------------------------------------------------------------------
// VolcanoEmbedder 对接豆包/火山引擎文本向量化 API。
// API 文档：https://www.volcengine.com/docs/82379/1099476
// ---------------------------------------------------------------------------

const (
	volcanoDefaultEndpoint = "https://ark.cn-beijing.volces.com"
	volcanoDefaultModel    = "doubao-embedding-large"
	volcanoDefaultDim      = 1024
	volcanoBatchSize       = 32
)

type VolcanoEmbedder struct {
	apiKey   string
	endpoint string
	model    string
	dim      int
	client   *http.Client
	fallback Embedder
}

// NewVolcanoEmbedder 根据 EmbedderConfig 构造 VolcanoEmbedder。
// 若 cfg 为 nil 或 apiKey 为空，则自动降级返回 HashEmbedder。
func NewVolcanoEmbedder(cfg *settings.EmbedderConfig) Embedder {
	if cfg == nil || cfg.APIKey == "" {
		return NewHashEmbedder(hashEmbedDim)
	}
	endpoint := cfg.Endpoint
	if endpoint == "" {
		endpoint = volcanoDefaultEndpoint
	}
	model := cfg.Model
	if model == "" {
		model = volcanoDefaultModel
	}
	dim := cfg.Dim
	if dim <= 0 {
		dim = volcanoDefaultDim
	}
	return &VolcanoEmbedder{
		apiKey:   cfg.APIKey,
		endpoint: endpoint,
		model:    model,
		dim:      dim,
		client:   &http.Client{Timeout: 10 * time.Second},
		fallback: NewHashEmbedder(dim),
	}
}

func (e *VolcanoEmbedder) Dim() int { return e.dim }

// Embed 分批调用 API，单批失败自动降级为 fallback。
func (e *VolcanoEmbedder) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, nil
	}
	result := make([][]float32, len(texts))
	for i := 0; i < len(texts); i += volcanoBatchSize {
		end := i + volcanoBatchSize
		if end > len(texts) {
			end = len(texts)
		}
		batch := texts[i:end]
		vecs, err := e.embedBatch(ctx, batch)
		if err != nil {
			// 降级：对本批使用 HashEmbedder，流程不中断
			fb, _ := e.fallback.Embed(ctx, batch)
			copy(result[i:end], fb)
			continue
		}
		copy(result[i:end], vecs)
	}
	return result, nil
}

type volcanoEmbedRequest struct {
	Model string   `json:"model"`
	Input []string `json:"input"`
}

type volcanoEmbedResponse struct {
	Data []struct {
		Index     int       `json:"index"`
		Embedding []float32 `json:"embedding"`
	} `json:"data"`
	Error *struct {
		Message string `json:"message"`
		Code    string `json:"code"`
	} `json:"error,omitempty"`
}

func (e *VolcanoEmbedder) embedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	body, err := json.Marshal(volcanoEmbedRequest{Model: e.model, Input: texts})
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		e.endpoint+"/api/v3/embeddings", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+e.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := e.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("volcano embed API status %d: %s", resp.StatusCode, string(respBody))
	}

	var result volcanoEmbedResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, err
	}
	if result.Error != nil {
		return nil, fmt.Errorf("volcano embed API error %s: %s", result.Error.Code, result.Error.Message)
	}

	vecs := make([][]float32, len(texts))
	for _, d := range result.Data {
		if d.Index < len(vecs) {
			vecs[d.Index] = d.Embedding
		}
	}
	// 对 API 未返回的索引用 fallback 补全
	for i, v := range vecs {
		if v == nil {
			fb, _ := e.fallback.Embed(ctx, []string{texts[i]})
			if len(fb) > 0 {
				vecs[i] = fb[0]
			}
		}
	}
	return vecs, nil
}
