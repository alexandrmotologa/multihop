package vector

import (
	"container/heap"
	"encoding/binary"
	"errors"
	"math"
)

var (
	ErrDimensionMismatch = errors.New("vector dimension mismatch")
	ErrEmptyVector       = errors.New("cannot calculate similarity on empty vector")
)

// CosineSimilarity computes cosine similarity between two float32 vectors.
// Operates in a single pass with loop unrolling for high throughput.
func CosineSimilarity(a, b []float32) float32 {
	n := len(a)
	if n != len(b) || n == 0 {
		return 0
	}

	var dot, normA, normB float32
	i := 0

	// 4x unrolled loop
	for ; i <= n-4; i += 4 {
		a0, a1, a2, a3 := a[i], a[i+1], a[i+2], a[i+3]
		b0, b1, b2, b3 := b[i], b[i+1], b[i+2], b[i+3]

		dot += a0*b0 + a1*b1 + a2*b2 + a3*b3
		normA += a0*a0 + a1*a1 + a2*a2 + a3*a3
		normB += b0*b0 + b1*b1 + b2*b2 + b3*b3
	}

	// Remainder
	for ; i < n; i++ {
		ai, bi := a[i], b[i]
		dot += ai * bi
		normA += ai * ai
		normB += bi * bi
	}

	if normA == 0 || normB == 0 {
		return 0
	}

	sim := dot / (float32(math.Sqrt(float64(normA))) * float32(math.Sqrt(float64(normB))))
	if sim > 1.0 {
		return 1.0
	}
	if sim < -1.0 {
		return -1.0
	}
	return sim
}

// NormalizeL2 scales a vector in-place to unit norm.
func NormalizeL2(v []float32) {
	var sum float32
	for _, x := range v {
		sum += x * x
	}
	if sum == 0 {
		return
	}
	norm := float32(math.Sqrt(float64(sum)))
	inv := 1.0 / norm
	for i := range v {
		v[i] *= inv
	}
}

// EncodeVector serializes a float32 slice into a little-endian byte array.
func EncodeVector(v []float32) []byte {
	buf := make([]byte, len(v)*4)
	for i, f := range v {
		binary.LittleEndian.PutUint32(buf[i*4:(i+1)*4], math.Float32bits(f))
	}
	return buf
}

// DecodeVector deserializes a little-endian byte array back into a float32 slice.
func DecodeVector(buf []byte) ([]float32, error) {
	if len(buf)%4 != 0 {
		return nil, errors.New("corrupt vector buffer: length not divisible by 4")
	}
	n := len(buf) / 4
	v := make([]float32, n)
	for i := 0; i < n; i++ {
		bits := binary.LittleEndian.Uint32(buf[i*4 : (i+1)*4])
		v[i] = math.Float32frombits(bits)
	}
	return v, nil
}

// ScoredItem represents an item scored by similarity.
type ScoredItem struct {
	ID    string
	Score float32
}

// MinHeap implements a min-heap for Top-K scoring.
type MinHeap []ScoredItem

func (h MinHeap) Len() int           { return len(h) }
func (h MinHeap) Less(i, j int) bool { return h[i].Score < h[j].Score }
func (h MinHeap) Swap(i, j int)      { h[i], h[j] = h[j], h[i] }

func (h *MinHeap) Push(x interface{}) {
	*h = append(*h, x.(ScoredItem))
}

func (h *MinHeap) Pop() interface{} {
	old := *h
	n := len(old)
	item := old[n-1]
	*h = old[0 : n-1]
	return item
}

// TopK maintains a bounded set of the K highest-scoring items.
type TopK struct {
	k int
	h MinHeap
}

// NewTopK initializes a TopK collector for up to k elements.
func NewTopK(k int) *TopK {
	tk := &TopK{k: k, h: make(MinHeap, 0, k)}
	heap.Init(&tk.h)
	return tk
}

// Add inserts an item if it qualifies for the top K.
func (tk *TopK) Add(id string, score float32) {
	if tk.k <= 0 {
		return
	}
	if tk.h.Len() < tk.k {
		heap.Push(&tk.h, ScoredItem{ID: id, Score: score})
		return
	}
	if score > tk.h[0].Score {
		tk.h[0] = ScoredItem{ID: id, Score: score}
		heap.Fix(&tk.h, 0)
	}
}

// Results returns sorted items from highest score to lowest.
func (tk *TopK) Results() []ScoredItem {
	n := tk.h.Len()
	res := make([]ScoredItem, n)
	for i := n - 1; i >= 0; i-- {
		res[i] = heap.Pop(&tk.h).(ScoredItem)
	}
	return res
}
