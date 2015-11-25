package cdkey

import (
	"fmt"
	"math"
	"math/rand"
	"sort"
	"strings"
	"time"
)

const (
	charSet    = "0123456789ABCDEFGHJKMNPQRSTVWXYZ"
	charSetLen = len(charSet)
)

// NormalizeKey transforms a key to base32 format.
//
// All lowercase letters will be converted to uppercase, 'O'/'o' will be replaced
// by '0', 'L'/'l' and 'I'/'i' will be replaced by '1'.
//
// If the given key contains any non-digital non-alphbetic character or 'U'/'u',
// it returns an empty string and false.
func NormalizeKey(key string) (string, bool) {
	key = strings.Map(func(r rune) rune {
		switch {
		case r == 'O':
			return '0'
		case r == 'I' || r == 'L':
			return '1'
		case r >= '0' && r <= '9' || r >= 'A' && r <= 'Z':
			return r
		default:
			return '?'
		}
	}, strings.ToUpper(key))

	if strings.Contains(key, "?") {
		return "", false
	} else {
		return key, true
	}
}

func keyGen1(prefix string, rndLen int) string {
	s := []byte(prefix)
	for i := 0; i < rndLen; i++ {
		s = append(s, charSet[rand.Int()%charSetLen])
	}
	return string(s)
}

// KeyGenN generates a set of CDKEYs, which guaranteed to be unique and sorted.
//
// It returns error if 1) `prefix` is not valid base32 format; 2)
//	32^(keylen-len(prefix)) < size * 100,
// which means a randomly generated key has a chance more than 1% to be valid.
func KeyGenN(prefix string, keylen, size int) ([]string, error) {
	normalPrefix, ok := NormalizeKey(prefix)
	if !ok {
		return nil, errInvalidPrefix.affix(fmt.Sprintf("prefix:%v", prefix))
	}

	rndLen := keylen - len(normalPrefix)
	if math.Pow(float64(charSetLen), float64(rndLen)) < float64(size*100) {
		return nil, errKeylenTooShort.affix(fmt.Sprintf("keylen:%v, rndLen:%v, size:%v", keylen, rndLen, size))
	}

	rand.Seed(time.Now().Unix())

	m := make(map[string]struct{})
	for len(m) < size {
		m[keyGen1(normalPrefix, rndLen)] = struct{}{}
	}

	var keys []string
	for k := range m {
		keys = append(keys, k)
	}

	sort.Strings(keys)

	return keys, nil
}
