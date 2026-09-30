package cli

import (
	"encoding/json"
	"fmt"
	"qmicode-go/codelib/session"
	"strings"
)

func color(role string) string {
	switch role {
	case "user":
		return Cyan
	case "assistant":
		return LightGray + Bold
	}
	return ""
}

func MessageList2String(sess *session.LLMSession) (string, error) {
	data, err := json.Marshal(sess.EntireMessages)
	if err != nil {
		return "", err
	}

	var context []map[string]any
	json.Unmarshal(data, &context)

	var res strings.Builder

	for _, msg := range context {
		role, ok := msg["role"]
		if ok {
			if role == "user" || role == "assistant" {
				if content, ok := msg["content"]; ok {
					res.WriteString(fmt.Sprintf("%s%s%s\n", color(role.(string)), content, Reset))
					continue
				}
			} else if role == "compact" {
				if content, ok := msg["content"]; ok {
					res.WriteString(fmt.Sprintf("%s%sCompact:%s\n", Green, Bold, Reset))
					res.WriteString(fmt.Sprintf("%s%s%s\n", color("assistant"), content, Reset))
					continue
				}
			}
		}
	}

	return res.String(), nil
}
