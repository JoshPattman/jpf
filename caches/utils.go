package caches

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"

	"github.com/JoshPattman/jpf"
)

func HashMessages(salt string, inputs []jpf.Message) string {
	s := &strings.Builder{}
	s.WriteString(salt)
	s.WriteString("Messages")
	for _, msg := range inputs {
		s.WriteString(msg.Role.String())
		s.WriteString(msg.Content)
		for _, img := range msg.Images {
			imgString, err := img.ToBase64Encoded(false)
			if err != nil {
				panic(err)
			}
			s.WriteString(imgString)
		}
	}
	src := s.String()
	hasher := sha256.New()
	hasher.Write([]byte(src))
	hashBytes := hasher.Sum(nil)
	return hex.EncodeToString(hashBytes)
}
