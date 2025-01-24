package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"github.com/sbreitf1/gohome/internal/pkg/stdio"
)

type cacheData struct {
	Entries   []Entry
	FlexiTime time.Duration
	Time      time.Time
}

func ReadCache() ([]Entry, time.Duration, time.Time, bool, error) {
	configDir := getConfigDir()
	cacheFile := filepath.Join(configDir, "cache.json")

	data, err := os.ReadFile(cacheFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, 0, time.Time{}, false, nil
		}
		return nil, 0, time.Time{}, false, err
	}

	var cd cacheData
	if err := json.Unmarshal(data, &cd); err != nil {
		return nil, 0, time.Time{}, false, err
	}

	now := time.Now()
	if cd.Time.Year() != now.Year() || cd.Time.Month() != now.Month() || cd.Time.Day() != now.Day() {
		stdio.Debug("cache is for another day")
		return nil, 0, time.Time{}, false, nil
	}
	maxCacheAge := time.Duration(cli.Show.CacheTimeSeconds) * time.Second
	if cd.Time.Before(now.Add(-maxCacheAge)) {
		stdio.Debug("cache is older than max age of %v", maxCacheAge)
		return nil, 0, time.Time{}, false, nil
	}
	return cd.Entries, cd.FlexiTime, cd.Time, true, nil
}

func WriteCache(entries []Entry, flexiTime time.Duration) error {
	configDir := getConfigDir()
	cacheFile := filepath.Join(configDir, "cache.json")

	data, err := json.MarshalIndent(cacheData{
		Entries:   entries,
		FlexiTime: flexiTime,
		Time:      time.Now(),
	}, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(cacheFile, data, os.ModePerm)
}
