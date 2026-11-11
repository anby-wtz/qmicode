package cli

import (
	"encoding/base64"
	"net/http"
	"os"

	"github.com/openai/openai-go"
)

func GetImageBase64(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}

	b64 := base64.StdEncoding.EncodeToString(data)
	mime := http.DetectContentType(data)

	return "data:" + mime + ";base64," + b64, nil
}

func GetOpenAIMsg(prompt string, attachImages []string) (openai.ChatCompletionMessageParamUnion, error) {
	parts := make([]openai.ChatCompletionContentPartUnionParam, 0)
	if prompt != "" {
		parts = append(parts, openai.TextContentPart(prompt))
	}
	for _, path := range attachImages {
		b64, err := GetImageBase64(path)
		if err != nil {
			return openai.ChatCompletionMessageParamUnion{}, err
		}
		parts = append(parts, openai.ImageContentPart(openai.ChatCompletionContentPartImageImageURLParam{
			URL:    b64,
			Detail: string(openai.ImageURLDeltaDetailAuto),
		}))
	}
	return openai.UserMessage(parts), nil
}
