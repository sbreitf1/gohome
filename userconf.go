package main

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type UserConfig struct {
	TargetTimeStr string `json:"TargetTime"`
}

func ReadUserConfig() (UserConfig, error) {
	configDir := getConfigDir()
	userConfFile := filepath.Join(configDir, "userconfig.json")

	data, err := os.ReadFile(userConfFile)
	if err != nil {
		if os.IsNotExist(err) {
			return UserConfig{}, nil
		}
		return UserConfig{}, err
	}

	var usrConf UserConfig
	if err := json.Unmarshal(data, &usrConf); err != nil {
		return UserConfig{}, err
	}

	return usrConf, nil
}

func WriteUserConfig(usrConf UserConfig) error {
	configDir := getConfigDir()
	userConfFile := filepath.Join(configDir, "userconfig.json")

	data, err := json.Marshal(&usrConf)
	if err != nil {
		return err
	}

	return os.WriteFile(userConfFile, data, os.ModePerm)
}
