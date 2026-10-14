package checksum

import (
	"crypto/md5"    // #nosec: 501 -- not used for security.
	"crypto/sha1"   // #nosec: 501 -- not used for security.
	"crypto/sha256" // #nosec: 501 -- not used for security.
	"crypto/sha512" // #nosec: 501 -- not used for security.
	"encoding/hex"
	"fmt"
	"hash"
	"io"
	"os"
)

// GenerateHash returns a hexadecimal encoded hash string generated from the
// contents of the file at path.
func GenerateHash(path, alg string) (string, error) {
	var h hash.Hash

	switch alg {
	case "MD5":
		h = md5.New() // #nosec: G401 -- not used for security.
	case "SHA-1":
		h = sha1.New() // #nosec: G401 -- not used for security.
	case "SHA-256":
		h = sha256.New() // #nosec: G401 -- not used for security.
	case "SHA-512":
		h = sha512.New() // #nosec: G401 -- not used for security.
	default:
		return "", fmt.Errorf("hash algorithm %q is not supported", alg)
	}

	f, err := os.Open(path) // #nosec: G304 -- trusted path.
	if err != nil {
		return "", fmt.Errorf("open file: %v", err)
	}
	defer f.Close()

	if _, err := io.Copy(h, f); err != nil {
		return "", fmt.Errorf("copy contents: %v", err)
	}

	return hex.EncodeToString(h.Sum(nil)), nil
}
