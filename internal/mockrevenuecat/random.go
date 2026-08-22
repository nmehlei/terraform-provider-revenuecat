package mockrevenuecat

import (
	"crypto/rand"
	"encoding/hex"
)

// randomSuffix returns a short random string, so generated identifiers are not
// guessable from a counter alone. A test that hardcodes an expected identifier
// then fails loudly instead of passing on a coincidence.
func randomSuffix() string {
	buf := make([]byte, 4)
	if _, err := rand.Read(buf); err != nil {
		return "0000"
	}
	return hex.EncodeToString(buf)
}
