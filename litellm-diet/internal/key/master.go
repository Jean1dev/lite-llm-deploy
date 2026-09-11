package key

import "crypto/subtle"

func Master(presented, configured string) bool {
	if presented == "" || configured == "" {
		return false
	}
	if len(presented) != len(configured) {
		subtle.ConstantTimeCompare([]byte(presented), []byte(presented))
		return false
	}
	return subtle.ConstantTimeCompare([]byte(presented), []byte(configured)) == 1
}
