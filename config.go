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
	return MatrixConfig{
		Host: "",
		User: "",
		Pass: "",
	}, nil
}

// GetDormaConfig returns host, user and password for Dorma from configuration or user input.
func GetDormaConfig() (DormaConfig, error) {
	configDir, err := getConfigDir()
	if err != nil {
		return DormaConfig{}, err
	}

	configFile := filepath.Join(configDir, "dorma.json")
	data, err := ioutil.ReadFile(configFile)
	if err != nil {
		if os.IsNotExist(err) {
			config, err := enterDormaConfig()
			if err != nil {
				return DormaConfig{}, err
			}

			if err := os.MkdirAll(configDir, os.ModePerm); err != nil {
				console.Printlnf("Failed to store configuration: %s", err.Error())
			}
			if err := jcrypt.MarshalToFile(configFile, &config, &jcrypt.Options{GetKeyHandler: jcrypt.StaticKey(key)}); err != nil {
				console.Printlnf("Failed to store configuration: %s", err.Error())
			}
			return config, nil
		}
		return DormaConfig{}, err
	}

	var config DormaConfig
	if err := jcrypt.Unmarshal(data, &config, &jcrypt.Options{GetKeyHandler: jcrypt.StaticKey(key)}); err != nil {
		return DormaConfig{}, err
	}

	return config, nil
}

func enterDormaConfig() (DormaConfig, error) {
	console.Printlnf("Please enter your Dorma configuration below:")
	console.Print("Host> ")
	host, err := console.ReadLine()
	if err != nil {
		return DormaConfig{}, err
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
		return DormaConfig{}, err
	}

	console.Print("Pass> ")
	pass, err := console.ReadPassword()
	if err != nil {
		return DormaConfig{}, err
	}

	return DormaConfig{host, user, pass}, nil
}

func getOldConfigDir() (string, error) {
	usr, err := user.Current()
	if err != nil {
		return "", err
	}
	return path.Join(usr.HomeDir, ".dorma"), nil
}

func hasOldConfig() bool {
	configDir, err := getOldConfigDir()
	if err != nil {
		return false
	}

	if _, err := os.Stat(filepath.Join(configDir, "app-hosts")); err != nil {
		return false
	}

	if _, err := os.Stat(filepath.Join(configDir, "host-credentials")); err != nil {
		if os.IsNotExist(err) {
			if _, err := os.Stat(filepath.Join(configDir, "host-credentials.gpg")); err != nil {
				return false
			}
			return true
		}
		return false
	}

	return true
}
