package config

import (
	"os"
	"path/filepath"
)

const (
	appName       = "textdrain"
	configDirName = ".config"
	cacheDirName  = ".cache"
	jobsDirName   = "jobs"
	modelsDirName = "models"
)

type Paths struct {
	ConfigDir  string
	ConfigFile string
	CacheDir   string
	JobsDir    string
	ModelsDir  string
}

func DefaultPaths() Paths {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		homeDir = "."
	}

	configDir := filepath.Join(homeDir, configDirName, appName)
	cacheDir := filepath.Join(homeDir, cacheDirName, appName)

	return Paths{
		ConfigDir:  configDir,
		ConfigFile: filepath.Join(configDir, "config.toml"),
		CacheDir:   cacheDir,
		JobsDir:    filepath.Join(cacheDir, jobsDirName),
		ModelsDir:  filepath.Join(cacheDir, modelsDirName),
	}
}
