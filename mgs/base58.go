package mgs

import (
	"errors"
	"fmt"
	"math/big"
	"strings"
)

const base58Alphabet = "123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz"

var (
	base58Radix = big.NewInt(58)
	base58Index [256]int16
)

func init() {
	for index := range base58Index {
		base58Index[index] = -1
	}
	for index, char := range []byte(base58Alphabet) {
		base58Index[char] = int16(index)
	}
}

func encodeBase58(input []byte) string {
	if len(input) == 0 {
		return ""
	}
	value := new(big.Int).SetBytes(input)
	mod := new(big.Int)
	var reversed []byte
	for value.Sign() > 0 {
		value.QuoRem(value, base58Radix, mod)
		reversed = append(reversed, base58Alphabet[mod.Int64()])
	}
	for _, b := range input {
		if b != 0 {
			break
		}
		reversed = append(reversed, base58Alphabet[0])
	}
	for left, right := 0, len(reversed)-1; left < right; left, right = left+1, right-1 {
		reversed[left], reversed[right] = reversed[right], reversed[left]
	}
	return string(reversed)
}

func decodeBase58(encoded string) ([]byte, error) {
	if encoded == "" {
		return nil, errors.New("empty base58 value")
	}
	value := new(big.Int)
	for index := 0; index < len(encoded); index++ {
		char := encoded[index]
		if base58Index[char] < 0 {
			return nil, fmt.Errorf("invalid base58 character %q", char)
		}
		value.Mul(value, base58Radix)
		value.Add(value, big.NewInt(int64(base58Index[char])))
	}
	decoded := value.Bytes()
	zeros := len(encoded) - len(strings.TrimLeft(encoded, "1"))
	result := make([]byte, zeros+len(decoded))
	copy(result[zeros:], decoded)
	return result, nil
}

func encodeMultibase(input []byte) string {
	return "z" + encodeBase58(input)
}

func decodeMultibase(value string) ([]byte, error) {
	if len(value) < 2 || value[0] != 'z' {
		return nil, errors.New("value must use multibase base58btc (z prefix)")
	}
	return decodeBase58(value[1:])
}
