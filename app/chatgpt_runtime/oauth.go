package chatgpt_runtime

import (
	"context"

	"qmicode-go/app/config"

	"github.com/sonnes/gopi/pkg/ai/chatgpt"
)

func OAuth(ctx context.Context, open func(url string) error) error {
	conf := config.DefaultConfig
	path := conf.GetChatgptAuthPath()

	// 传入已有账号，复用上次注册得到的 ClientID/HostID
	var prior *chatgpt.Account
	if acc, err := chatgpt.LoadAccount(path); err == nil {
		prior = &acc
	}

	auth := chatgpt.Auth{}
	account, err := auth.SignIn(ctx, chatgpt.SignInConfig{
		AppName: config.AppName,
		HostID:  chatgpt.NewHostID(),
		Port:    chatgpt.DefaultPort,
		Open:    open,
	}, prior)
	if err != nil {
		return err
	}

	return chatgpt.SaveAccount(path, account)
}

func LoadAccount(conf config.Config) (chatgpt.Account, error) {
	return chatgpt.LoadAccount(conf.GetChatgptAuthPath())
}

func AccountEmail(conf config.Config) (string, error) {
	acc, err := LoadAccount(conf)
	if err != nil {
		return "", err
	}
	return acc.Email, nil
}
