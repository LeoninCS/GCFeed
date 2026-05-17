package domainembedding

import (
	"hash/fnv"
	"math"
	"strings"
	"unicode"
)

const HashNgramModel = "hash-ngram-v1"
const HashNgramDimension = 128

// Vectorizer 把文本转换为固定维度向量。
type Vectorizer interface {
	Model() string
	Dimension() int
	Vectorize(text string) []float64
}

// HashNgramVectorizer 是零依赖的本地伪 embedding，适合先跑通推荐链路。
type HashNgramVectorizer struct {
	dimension int
}

// NewHashNgramVectorizer 创建默认 128 维 hash n-gram 向量器。
func NewHashNgramVectorizer() *HashNgramVectorizer {
	return &HashNgramVectorizer{dimension: HashNgramDimension}
}

func (v *HashNgramVectorizer) Model() string {
	return HashNgramModel
}

func (v *HashNgramVectorizer) Dimension() int {
	if v == nil || v.dimension <= 0 {
		return HashNgramDimension
	}
	return v.dimension
}

// Vectorize 使用字符 n-gram 和 token 特征生成稳定向量，并做 L2 归一化。
func (v *HashNgramVectorizer) Vectorize(text string) []float64 {
	dimension := v.Dimension()
	vector := make([]float64, dimension)
	normalized := normalizeText(text)
	if normalized == "" {
		return vector
	}

	tokens := strings.Fields(normalized)
	for _, token := range tokens {
		addFeature(vector, "tok:"+token, 1.0)
	}

	runes := []rune(strings.ReplaceAll(normalized, " ", ""))
	for n := 2; n <= 3; n++ {
		if len(runes) < n {
			continue
		}
		for i := 0; i+n <= len(runes); i++ {
			addFeature(vector, string(runes[i:i+n]), 1.0)
		}
	}

	normalizeVector(vector)
	return vector
}

// CosineSimilarity 计算两个已归一化或未归一化向量的余弦相似度。
func CosineSimilarity(left []float64, right []float64) (float64, error) {
	if len(left) != len(right) {
		return 0, ErrDimensionMismatch
	}
	var dot float64
	var leftNorm float64
	var rightNorm float64
	for i := range left {
		dot += left[i] * right[i]
		leftNorm += left[i] * left[i]
		rightNorm += right[i] * right[i]
	}
	if leftNorm == 0 || rightNorm == 0 {
		return 0, nil
	}
	return dot / (math.Sqrt(leftNorm) * math.Sqrt(rightNorm)), nil
}

func normalizeText(text string) string {
	text = strings.ToLower(strings.TrimSpace(text))
	if text == "" {
		return ""
	}

	var builder strings.Builder
	previousSpace := true
	for _, r := range text {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			builder.WriteRune(r)
			previousSpace = false
			continue
		}
		if unicode.IsSpace(r) || unicode.IsPunct(r) || unicode.IsSymbol(r) {
			if !previousSpace {
				builder.WriteRune(' ')
				previousSpace = true
			}
		}
	}
	return strings.TrimSpace(builder.String())
}

func addFeature(vector []float64, feature string, weight float64) {
	if len(vector) == 0 || feature == "" {
		return
	}
	hasher := fnv.New64a()
	_, _ = hasher.Write([]byte(feature))
	hash := hasher.Sum64()
	index := int(hash % uint64(len(vector)))
	sign := 1.0
	if (hash>>63)&1 == 1 {
		sign = -1.0
	}
	vector[index] += sign * weight
}

func normalizeVector(vector []float64) {
	var sum float64
	for _, value := range vector {
		sum += value * value
	}
	if sum == 0 {
		return
	}
	norm := math.Sqrt(sum)
	for i := range vector {
		vector[i] = vector[i] / norm
	}
}
