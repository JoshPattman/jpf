package jpf

import (
	"errors"
	"math"
)

type EmbedCaller = Caller[string, []float64]

// CosineSimilarity takes the cosine similarity between two vectors.
func CosineSimilarity(vec1, vec2 []float64) (float64, error) {
	if len(vec1) != len(vec2) {
		return 0, errors.New("vectors must be of the same length")
	}

	var dotProduct, magnitudeVec1, magnitudeVec2 float64

	for i := range vec1 {
		dotProduct += vec1[i] * vec2[i]
		magnitudeVec1 += vec1[i] * vec1[i]
		magnitudeVec2 += vec2[i] * vec2[i]
	}

	if magnitudeVec1 == 0 || magnitudeVec2 == 0 {
		return 0, errors.New("one of the vectors has zero magnitude")
	}

	return dotProduct / (math.Sqrt(magnitudeVec1) * math.Sqrt(magnitudeVec2)), nil
}
