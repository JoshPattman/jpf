package jpf

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

type ModelResponseCache interface {
	GetCachedResponse(salt string, inputs []Message) (bool, []Message, Message, error)
	SetCachedResponse(salt string, inputs []Message, aux []Message, out Message) error
}

func HashMessages(salt string, inputs []Message) string {
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
