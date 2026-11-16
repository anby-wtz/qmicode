package cli

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"os"
	"qmicode-go/codelib/session"
	"strings"
)

func GetImageBase64(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}

	mime := http.DetectContentType(data)
	if !strings.HasPrefix(mime, "image/") && mime != "application/pdf" {
		return "", fmt.Errorf("%s: unsupported media type %q", path, mime)
	}

	b64 := base64.StdEncoding.EncodeToString(data)

	return "data:" + mime + ";base64," + b64, nil
}

func GetMsg(prompt string, attachImages []string) (*session.LLMMessage, error) {
	images := []string{}
	for _, path := range attachImages {
		b64, err := GetImageBase64(path)
		if err != nil {
			return &session.LLMMessage{}, err
		}
		images = append(images, b64)
	}
	return session.NewUserMessage(&prompt, images), nil
}
