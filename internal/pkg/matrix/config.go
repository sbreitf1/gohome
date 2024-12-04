package main

import (
	"os"
	"os/user"
	"path"
	"path/filepath"
	"strings"

	"github.com/sbreitf1/gohome/internal/pkg/jcrypt"

	"github.com/adrg/xdg"
	"github.com/sbreitf1/go-console"
)

var (
	key = []byte{42, 13, 37}
)

type configSources struct {
	XDGDir  string
	HomeDir string
}

func (confSrc configSources) IsZero() bool {
	return len(confSrc.XDGDir) == 0 && len(confSrc.HomeDir) == 0
}

func getAvailableConfigDirs() configSources {
	var confSrc configSources

	xdgConfigDir := getXDGConfigDir()
	xdgConfigFile := filepath.Join(xdgConfigDir, "matrix.json")
	if _, err := os.Stat(xdgConfigFile); err == nil {
		// file exists and can be accessed => nothing to do
		confSrc.XDGDir = xdgConfigDir
	}

	if homeConfigDir, err := getHomeConfigDir(); err == nil {
		homeConfigFile := filepath.Join(homeConfigDir, "matrix.json")
		if _, err := os.Stat(homeConfigFile); err == nil {
			// file exists and can be accessed => nothing to do
			confSrc.HomeDir = homeConfigDir
		}
	}

	return confSrc
}

func migrateOldConfig() error {
	confSrc := getAvailableConfigDirs()
	if confSrc.IsZero() {
		// nothing to migrate
		return nil
	}
	if len(confSrc.XDGDir) > 0 {
		// config already present at correct location
		return nil
	}

	dstDir := getXDGConfigDir()

	// need to migrate
	if err := os.MkdirAll(dstDir, os.ModePerm); err != nil {
		return err
	}

	rawData, err := os.ReadFile(filepath.Join(confSrc.HomeDir, "matrix.json"))
	if err != nil {
		return err
	}

	if err := os.WriteFile(filepath.Join(dstDir, "matrix.json"), rawData, os.ModePerm); err != nil {
		return err
	}

	if err := os.RemoveAll(confSrc.HomeDir); err != nil {
		return err
	}

	console.Printlnf("config migrated to %s", dstDir)
	return nil
}

func getHomeConfigDir() (string, error) {
	usr, err := user.Current()
	if err != nil {
		return "", err
	}
	return path.Join(usr.HomeDir, ".gohome"), nil
}

func getXDGConfigDir() string {
	return path.Join(xdg.ConfigHome, "gohome")
}

func getConfigDir() string {
	confSrc := getAvailableConfigDirs()
	if confSrc.IsZero() {
		// prefer xdg dir for new config
		return getXDGConfigDir()
	}

	if len(confSrc.XDGDir) > 0 {
		// prefer config from xdg dir
		return confSrc.XDGDir
	}
	// fall back to old config location
	return confSrc.HomeDir
}

func GetMatrixConfig() (MatrixConfig, error) {
	if err := migrateOldConfig(); err != nil {
		console.Printlnf("failed to migrate config: %s", err.Error())
	}

	configDir := getConfigDir()
	configFile := filepath.Join(configDir, "matrix.json")
	data, err := os.ReadFile(configFile)
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
