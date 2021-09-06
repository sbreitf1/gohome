package main

import (
	"io/ioutil"
	"os"
	"os/user"
	"path"
	"path/filepath"
	"strings"

	"github.com/sbreitf1/go-console"
	"github.com/sbreitf1/go-jcrypt"
)

var (
	key = []byte{42, 13, 37}
)

func getConfigDir() (string, error) {
	usr, err := user.Current()
	if err != nil {
		return "", err
	}
	return path.Join(usr.HomeDir, ".gohome"), nil
}

func GetMatrixConfig() (MatrixConfig, error) {
	configDir, err := getConfigDir()
	if err != nil {
		return MatrixConfig{}, err
	}

	configFile := filepath.Join(configDir, "matrix.json")
	data, err := ioutil.ReadFile(configFile)
	if err != nil {
		if os.IsNotExist(err) {
			config, err := enterMatrixConfig()
			if err != nil {
				return MatrixConfig{}, err
			}

			if err := os.MkdirAll(configDir, os.ModePerm); err != nil {
				console.Printlnf("Failed to store configuration: %s", err.Error())
			}
			if err := jcrypt.MarshalToFile(configFile, &config, &jcrypt.Options{GetKeyHandler: jcrypt.StaticKey(key)}); err != nil {
				console.Printlnf("Failed to store configuration: %s", err.Error())
			}
			return config, nil
		}
		return MatrixConfig{}, err
	}

	var config MatrixConfig
	if err := jcrypt.Unmarshal(data, &config, &jcrypt.Options{GetKeyHandler: jcrypt.StaticKey(key)}); err != nil {
		return MatrixConfig{}, err
	}

	return config, nil
}

func enterMatrixConfig() (MatrixConfig, error) {
	console.Printlnf("Please enter your Matrix configuration below:")
	console.Print("Host> ")
	host, err := console.ReadLine()
	if err != nil {
		return MatrixConfig{}, err
	}

	// ensure protocol is appended
	if !strings.HasPrefix(strings.ToLower(host), "http://") && !strings.HasPrefix(strings.ToLower(host), "https://") {
		host = "https://" + host
	}
	// and now remove path information
	protIndex := strings.Index(host, "://")
	if index := strings.Index(host[protIndex+3:], "/"); index >= 0 {
		host = host[:index+protIndex+3]
	}

	console.Print("User> ")
	user, err := console.ReadLine()
	if err != nil {
		return MatrixConfig{}, err
	}

	console.Print("Pass> ")
	pass, err := console.ReadPassword()
	if err != nil {
		return MatrixConfig{}, err
	}

	return MatrixConfig{host, user, pass}, nil
}
