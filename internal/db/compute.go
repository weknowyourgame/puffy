package db

import "errors"
import "math"

/*
* Matrix multiplication of 2 1D vectors
*/
func DotProduct(a, b []float32) (float32, error) {
	if len(a) != len(b) {
		return 0.0, errors.New("vector dimensions are not compatible for multiplication")
	}
	result := float32(0.0)
	for i := range a {
		result += a[i] * b[i]
	}
	return result, nil
}

/*
* Cosine similarity
* cos(a,b) = dot(a,b) / (‖a‖ · ‖b‖), where ‖a‖ = sqrt(dot(a,a)) and ‖b‖ = sqrt(dot(b,b))
*/
func CosineSimilarity(a,b []float32) (float32, error) {
	if len(a) != len(b) {
		return 0.0, errors.New("vector dimensions are not compatible for cosine similarity")
	}

    dotProduct, err := DotProduct(a, b)
    if err != nil {
        return 0.0, err
    }

    num1, _ := DotProduct(a, a)
    num2, _ := DotProduct(b, b)

	// Math only accepts float64
    sq1 := math.Sqrt(float64(num1))
    sq2 := math.Sqrt(float64(num2))

	// type cast to 32
	return float32(float64(dotProduct) / (sq1 * sq2)), nil
}
