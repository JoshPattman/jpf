package caches

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/JoshPattman/jpf"
)

func HashMessages(salt string, inputs []jpf.Message) string {
	s := &strings.Builder{}
	s.WriteString(salt)
	s.WriteString("Messages")
	for _, msg := range inputs {
		s.WriteString(messageToString(msg) + ";")
	}
	src := s.String()
	hasher := sha256.New()
	hasher.Write([]byte(src))
	hashBytes := hasher.Sum(nil)
	return hex.EncodeToString(hashBytes)
}

func messageToString(msg jpf.Message) string {
	switch msg := msg.(type) {
	case jpf.UserMessage:
		return fmt.Sprintf("user:%s:%s", msg.Content, imageAttachmentsToString(msg.Images))
	case jpf.AssistantMessage:
		return fmt.Sprintf("assistant:%s:%v", msg.Content, msg.ToolCalls)
	case jpf.DeveloperMessage:
		return fmt.Sprintf("developer:%s", msg.Content)
	case jpf.SystemMessage:
		return fmt.Sprintf("system:%s", msg.Content)
	case jpf.ToolResultMessage:
		return fmt.Sprintf("result:%s:%s", msg.CallID, msg.Result)
	default:
		panic("unreachable")
	}
}

func imageAttachmentsToString(attachments []jpf.ImageAttachment) string {
	ss := []string{}
	for _, img := range attachments {
		imgString, err := img.ToBase64Encoded(false)
		if err != nil {
			panic(err)
		}
		ss = append(ss, imgString)
	}
	return strings.Join(ss, "&")
}
