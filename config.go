package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"os/user"
	"path"
	"path/filepath"
	"strings"

	"github.com/sbreitf1/go-console"
	"github.com/sbreitf1/go-jcrypt"
	"github.com/sbreitf1/gpgutil"
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
	// fallback-solution: check for old config and migrate
	if hasOldConfig() {
		config, err := importOldDormaConfig()
		if err == nil {
			console.Println("Old Dorma configuration has been imported")
			return config, err
		}
		console.Println("Failed to migrate old Dorma config.")
	}

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

func importOldDormaConfig() (DormaConfig, error) {
	host, err := getOldDormaHost("gohome-app")
	if err != nil {
		return DormaConfig{}, fmt.Errorf("failed to read old Dorma host")
	}

	user, pass, err := getOldCredentials(host)
	if err != nil {
		return DormaConfig{}, fmt.Errorf("failed to read old Dorma credentials")
	}

	return DormaConfig{host, user, pass}, err
}

func getOldDormaHost(appID string) (string, error) {
	configDir, err := getOldConfigDir()
	if err != nil {
		return "", err
	}

	hostsFile := path.Join(configDir, "app-hosts")
	hosts, err := readOldAppHosts(hostsFile)
	if err != nil {
		return "", err
	}

	if host, ok := hosts[appID]; ok {
		if !strings.HasPrefix(strings.ToLower(host), "http://") && !strings.HasPrefix(strings.ToLower(host), "https://") {
			// backward-compatibility for missing protocols
			return "https://" + host, nil
		}
		return host, nil
	}

	return "", fmt.Errorf("no Dorma host defined")
}

func readOldAppHosts(file string) (map[string]string, error) {
	data, err := ioutil.ReadFile(file)
	if err != nil {
		if os.IsNotExist(err) {
			return make(map[string]string), nil
		}
		return nil, err
	}

	var hosts map[string]string
	if err := json.Unmarshal(data, &hosts); err != nil {
		return nil, err
	}

	return hosts, nil
}

type credential struct {
	User string `json:"user"`
	Pass string `json:"pass"`
}

func getOldCredentials(dormaHost string) (string, string, error) {
	configDir, err := getOldConfigDir()
	if err != nil {
		return "", "", err
	}

	credentialsFile := path.Join(configDir, "host-credentials")
	credentials, err := readOldHostCredentials(credentialsFile)
	if err != nil {
		return "", "", err
	}

	if c, ok := credentials[dormaHost]; ok {
		return c.User, c.Pass, nil
	}

	// try with missing protocol for backwards compatibility
	if strings.HasPrefix(strings.ToLower(dormaHost), "http://") {
		if c, ok := credentials[dormaHost[7:]]; ok {
			return c.User, c.Pass, nil
		}
	}
	if strings.HasPrefix(strings.ToLower(dormaHost), "https://") {
		if c, ok := credentials[dormaHost[8:]]; ok {
			return c.User, c.Pass, nil
		}
	}

	return "", "", fmt.Errorf("no Dorma credentials defined")
}

func readOldHostCredentials(file string) (map[string]credential, error) {
	var data []byte
	var err error
	if len(*argPGPKeyName) > 0 {
		_, gpgErr := os.Stat(file + ".gpg")
		if gpgErr == nil {
			key := gpgutil.MakeNamedKeySource(*argPGPKeyName, "")
			data, err = gpgutil.DecryptFileToByteSlice(file+".gpg", key, nil)
		}
	} else {
		_, gpgErr := os.Stat(file + ".gpg")
		if !os.IsNotExist(gpgErr) {
			return nil, fmt.Errorf("GPG key required, encrypted configuration exits")
		}
	}

	if data == nil && err == nil {
		data, err = ioutil.ReadFile(file)
	}

	if err != nil {
		if os.IsNotExist(err) {
			return make(map[string]credential), nil
		}
		return nil, err
	}

	var credentials map[string]credential
	if err := json.Unmarshal(data, &credentials); err != nil {
		return nil, err
	}

	return credentials, nil
}
