package login

import (
	"encoding/json"
	"os"
	"path"

	"github.com/YangchenYe323/boxtroll/internal/bilibili"
	"github.com/rs/zerolog/log"
)

func GetCachedCredential(dir string) (*bilibili.Credential, error) {
	path := path.Join(dir, "credential.json")

	bytes, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var credential bilibili.Credential
	if err := json.Unmarshal(bytes, &credential); err != nil {
		log.Error().Msg("credential.json 解析失败，删除可能损坏的文件")
		os.Remove(path)
		return nil, os.ErrNotExist
	}

	return &credential, nil
}

func SaveCredential(dir string, credential *bilibili.Credential) error {
	path := path.Join(dir, "credential.json")

	bytes, err := json.Marshal(credential)
	if err != nil {
		return err
	}

	return os.WriteFile(path, bytes, 0644)
}
