package jpf

import (
	"bytes"
	"crypto/sha256"
	"encoding/gob"
	"encoding/hex"
	"errors"
	"strings"
)

var ErrNoCache = errors.New("no cache for that hash")

type Cache interface {
	Set(key string, data []byte) error
	Get(key string) ([]byte, error)
}

func HashMessages(msgs []Message) string {
	s := &strings.Builder{}
	s.WriteString("Messages")
	for _, msg := range msgs {
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

func EncodeChatResult(result ChatResult) ([]byte, error) {
	blob := bytes.NewBuffer(nil)
	err := gob.NewEncoder(blob).Encode(result)
	if err != nil {
		return nil, err
	}
	return blob.Bytes(), nil
}

func DecodeChatResult(bs []byte) (ChatResult, error) {
	var result ChatResult
	err := gob.NewDecoder(bytes.NewBuffer(bs)).Decode(&result)
	if err != nil {
		return ChatResult{}, err
	}
	return result, nil
}

func EncodeEmbedResult(result []float64) ([]byte, error) {
	blob := bytes.NewBuffer(nil)
	err := gob.NewEncoder(blob).Encode(result)
	if err != nil {
		return nil, err
	}
	return blob.Bytes(), nil
}

func DecodeEmbedResult(bs []byte) ([]float64, error) {
	var result []float64
	err := gob.NewDecoder(bytes.NewBuffer(bs)).Decode(&result)
	if err != nil {
		return nil, err
	}
	return result, nil
}
