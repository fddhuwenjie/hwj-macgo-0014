package domain

import (
	"crypto/sha256"
	"fmt"
)

type SubjectDigest struct {
	Algorithm string
	Value     string
}

func NewSubjectDigest(subject string) SubjectDigest {
	h := sha256.Sum256([]byte(subject))
	return SubjectDigest{
		Algorithm: "SHA256",
		Value:     fmt.Sprintf("%x", h[:]),
	}
}

func (d SubjectDigest) String() string {
	return fmt.Sprintf("%s:%s", d.Algorithm, d.Value)
}
