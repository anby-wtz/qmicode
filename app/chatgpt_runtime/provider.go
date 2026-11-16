package chatgpt_runtime

import (
	"errors"
	"fmt"
	"sync"

	"qmicode-go/app/config"

	"github.com/sonnes/gopi/pkg/ai/chatgpt"
	"github.com/sonnes/gopi/pkg/ai/oauth"
)

var ErrNotSignedIn = errors.New("chatgpt: not signed in")

var (
	providerMu sync.Mutex
	providers  = map[string]*chatgpt.Provider{}
)

func providerWithSource(src oauth.Source, baseURL string) *chatgpt.Provider {
	var opts []chatgpt.Option
	if baseURL != "" {
		opts = append(opts, chatgpt.BaseURL(baseURL))
	}
	return chatgpt.New(src, opts...)
}

// provider 按 auth 文件与 baseURL 复用同一个 Provider，其 FileSource 自带 token 缓存与文件锁。
func provider(conf config.Config, baseURL string) *chatgpt.Provider {
	authPath := conf.GetChatgptAuthPath()
	key := authPath + "\x00" + baseURL

	providerMu.Lock()
	defer providerMu.Unlock()

	if p, ok := providers[key]; ok {
		return p
	}
	p := providerWithSource(chatgpt.FileSource(authPath, chatgpt.Auth{}), baseURL)
	providers[key] = p
	return p
}

func mapAuthError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, chatgpt.ErrNoAccount) || errors.Is(err, chatgpt.ErrSignInExpired) {
		return fmt.Errorf("%w: %v", ErrNotSignedIn, err)
	}
	return err
}

func checkSignedIn(conf config.Config) error {
	acc, err := LoadAccount(conf)
	if err != nil {
		return mapAuthError(err)
	}
	if !acc.SignedIn() {
		return fmt.Errorf("%w: no account in %s", ErrNotSignedIn, conf.GetChatgptAuthPath())
	}
	return nil
}
