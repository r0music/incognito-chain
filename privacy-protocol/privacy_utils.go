package privacy

import (
	"crypto/rand"
	"fmt"
)

// RandBytes generates random bytes
func RandBytes(n int) []byte {
	b := make([]byte, n)
	_, err := rand.Read(b)
	if err != nil {
		fmt.Println("error:", err)
		return nil
	}
	return b
}

// IsPowerOfTwo checks whether n is power of two or not
func IsPowerOfTwo(n int) bool {
	if n < 2 {
		return false
	}
	for n > 2 {
		if n%2 == 0 {
			n = n / 2
		} else {
			return false
		}
	}
	return true

}

// ConvertIntToBinary represents a integer number in binary
func ConvertIntToBinary(inum int, n int) []byte {
	binary := make([]byte, n)

	for i := n - 1; i >= 0; i-- {
		binary[i] = byte(inum % 2)
		inum = inum / 2
	}

	return binary
}
