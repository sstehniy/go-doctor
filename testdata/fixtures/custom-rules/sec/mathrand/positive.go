package mathrand

import "math/rand"

func GenerateToken() int {
	token := rand.Int() // want sec/math-rand-for-secret
	return token
}
