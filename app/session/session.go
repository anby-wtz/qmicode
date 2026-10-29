package session

import (
	"encoding/json"
	"os"
	"qmicode-go/app/config"
	coresess "qmicode-go/codelib/session"
)

func SaveSession(conf config.Config, sess *coresess.LLMSession) error {
	jsonBytes, err := json.Marshal(sess)
	if err != nil {
		return err
	}
	file, err := os.Create(conf.GetSessionPath(sess.Name))
	if err != nil {
		return err
	}
	defer file.Close()
	_, err = file.Write(jsonBytes)
	return err
}

func ListSessions(conf config.Config) ([]string, error) {
	files, err := os.ReadDir(conf.GetSessionDirPath())
	if err != nil {
		return nil, err
	}
	var sessions []string
	for _, file := range files {
		if !file.IsDir() {
			sessions = append(sessions, file.Name()[:len(file.Name())-5])
		}
	}
	return sessions, nil

}

// 加载并替换sess指针指向的内容
func LoadSession(conf config.Config, name string, sess *coresess.LLMSession) error {
	sess.Messages = nil
	sess.EntireMessages = nil
	file, err := os.Open(conf.GetSessionPath(name))
	if err != nil {
		return err
	}
	defer file.Close()
	decoder := json.NewDecoder(file)
	return decoder.Decode(sess)
}

func RenameSession(conf config.Config, sess *coresess.LLMSession, newName string) error {
	oldName := sess.Name
	sess.Name = newName
	return os.Rename(conf.GetSessionPath(oldName), conf.GetSessionPath(newName))
}
