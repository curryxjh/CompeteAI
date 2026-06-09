package memory

import (
	"context"
	"hash/fnv"
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
