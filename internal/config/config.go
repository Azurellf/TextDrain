package config

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

	"textdrain/internal/domain"
)

const (
	DefaultModel    = "small"
	DefaultLanguage = "auto"
)

var DefaultOutputFormats = []domain.OutputFormat{
	domain.OutputFormatTXT,
	domain.OutputFormatSRT,
	domain.OutputFormatVTT,
	domain.OutputFormatJSON,
}

type Config struct {
	Model                 string
	Language              string
	OutputFormats         []domain.OutputFormat
	KeepIntermediateFiles bool
	ModelDir              string
	JobsDir               string
}

type Overrides struct {
	Model                 *string
	Language              *string
	OutputFormats         *[]domain.OutputFormat
	KeepIntermediateFiles *bool
	ModelDir              *string
	JobsDir               *string
}

func Default(paths Paths) Config {
	return Config{
		Model:                 DefaultModel,
		Language:              DefaultLanguage,
		OutputFormats:         cloneOutputFormats(DefaultOutputFormats),
		KeepIntermediateFiles: false,
		ModelDir:              paths.ModelsDir,
		JobsDir:               paths.JobsDir,
	}
}

func Load(paths Paths, overrides Overrides) (Config, error) {
	cfg := Default(paths)

	if paths.ConfigFile != "" {
		fileCfg, err := loadFile(paths.ConfigFile)
		if err != nil {
			return Config{}, err
		}
		applyFileConfig(&cfg, fileCfg)
	}

	applyOverrides(&cfg, overrides)

	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}

	return cfg, nil
}

func (c Config) Validate() error {
	if c.Model == "" {
		return errors.New("config model cannot be empty")
	}
	if c.Language == "" {
		return errors.New("config language cannot be empty")
	}
	switch c.Language {
	case "auto", "zh", "en":
	default:
		return fmt.Errorf("unsupported language %q", c.Language)
	}
	if len(c.OutputFormats) == 0 {
		return errors.New("config output_formats cannot be empty")
	}
	if c.ModelDir == "" {
		return errors.New("config model_dir cannot be empty")
	}
	if c.JobsDir == "" {
		return errors.New("config jobs_dir cannot be empty")
	}

	for _, format := range c.OutputFormats {
		switch format {
		case domain.OutputFormatTXT, domain.OutputFormatSRT, domain.OutputFormatVTT, domain.OutputFormatJSON:
		default:
			return fmt.Errorf("unsupported output format %q", format)
		}
	}

	return nil
}

type fileConfig struct {
	modelSet                 bool
	model                    string
	languageSet              bool
	language                 string
	outputFormatsSet         bool
	outputFormats            []domain.OutputFormat
	keepIntermediateFilesSet bool
	keepIntermediateFiles    bool
	modelDirSet              bool
	modelDir                 string
	jobsDirSet               bool
	jobsDir                  string
}

func loadFile(path string) (fileConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return fileConfig{}, nil
		}
		return fileConfig{}, fmt.Errorf("read config file %s: %w", path, err)
	}

	cfg, err := parseTOMLConfig(string(data))
	if err != nil {
		return fileConfig{}, fmt.Errorf("parse config file %s: %w", path, err)
	}

	return cfg, nil
}

func parseTOMLConfig(input string) (fileConfig, error) {
	var cfg fileConfig
	scanner := bufio.NewScanner(strings.NewReader(input))
	lineNumber := 0

	for scanner.Scan() {
		lineNumber++
		line := stripComment(scanner.Text())
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "[") {
			return fileConfig{}, fmt.Errorf("line %d: tables are not supported in config.toml", lineNumber)
		}

		key, rawValue, ok := strings.Cut(line, "=")
		if !ok {
			return fileConfig{}, fmt.Errorf("line %d: expected key = value", lineNumber)
		}

		key = strings.TrimSpace(key)
		rawValue = strings.TrimSpace(rawValue)

		switch key {
		case "model":
			value, err := parseString(rawValue)
			if err != nil {
				return fileConfig{}, fmt.Errorf("line %d: %w", lineNumber, err)
			}
			cfg.modelSet = true
			cfg.model = value
		case "language":
			value, err := parseString(rawValue)
			if err != nil {
				return fileConfig{}, fmt.Errorf("line %d: %w", lineNumber, err)
			}
			cfg.languageSet = true
			cfg.language = value
		case "output_formats":
			values, err := parseStringArray(rawValue)
			if err != nil {
				return fileConfig{}, fmt.Errorf("line %d: %w", lineNumber, err)
			}
			cfg.outputFormatsSet = true
			cfg.outputFormats = make([]domain.OutputFormat, 0, len(values))
			for _, value := range values {
				cfg.outputFormats = append(cfg.outputFormats, domain.OutputFormat(value))
			}
		case "keep_intermediate_files":
			value, err := strconv.ParseBool(rawValue)
			if err != nil {
				return fileConfig{}, fmt.Errorf("line %d: keep_intermediate_files must be true or false", lineNumber)
			}
			cfg.keepIntermediateFilesSet = true
			cfg.keepIntermediateFiles = value
		case "model_dir":
			value, err := parseString(rawValue)
			if err != nil {
				return fileConfig{}, fmt.Errorf("line %d: %w", lineNumber, err)
			}
			cfg.modelDirSet = true
			cfg.modelDir = value
		case "jobs_dir":
			value, err := parseString(rawValue)
			if err != nil {
				return fileConfig{}, fmt.Errorf("line %d: %w", lineNumber, err)
			}
			cfg.jobsDirSet = true
			cfg.jobsDir = value
		default:
			return fileConfig{}, fmt.Errorf("line %d: unknown config key %q", lineNumber, key)
		}
	}

	if err := scanner.Err(); err != nil {
		return fileConfig{}, fmt.Errorf("scan config: %w", err)
	}

	return cfg, nil
}

func stripComment(line string) string {
	inString := false
	escaped := false

	for i, r := range line {
		if escaped {
			escaped = false
			continue
		}
		if r == '\\' && inString {
			escaped = true
			continue
		}
		if r == '"' {
			inString = !inString
			continue
		}
		if r == '#' && !inString {
			return line[:i]
		}
	}

	return line
}

func parseString(raw string) (string, error) {
	if len(raw) < 2 || raw[0] != '"' || raw[len(raw)-1] != '"' {
		return "", errors.New("value must be a quoted string")
	}

	value, err := strconv.Unquote(raw)
	if err != nil {
		return "", fmt.Errorf("invalid string value: %w", err)
	}

	return value, nil
}

func parseStringArray(raw string) ([]string, error) {
	if len(raw) < 2 || raw[0] != '[' || raw[len(raw)-1] != ']' {
		return nil, errors.New("value must be a string array")
	}

	body := strings.TrimSpace(raw[1 : len(raw)-1])
	if body == "" {
		return []string{}, nil
	}

	parts := splitArrayItems(body)
	values := make([]string, 0, len(parts))
	for _, part := range parts {
		value, err := parseString(strings.TrimSpace(part))
		if err != nil {
			return nil, err
		}
		values = append(values, value)
	}

	return values, nil
}

func splitArrayItems(body string) []string {
	var parts []string
	start := 0
	inString := false
	escaped := false

	for i, r := range body {
		if escaped {
			escaped = false
			continue
		}
		if r == '\\' && inString {
			escaped = true
			continue
		}
		if r == '"' {
			inString = !inString
			continue
		}
		if r == ',' && !inString {
			parts = append(parts, body[start:i])
			start = i + 1
		}
	}

	parts = append(parts, body[start:])
	return parts
}

func applyFileConfig(cfg *Config, fileCfg fileConfig) {
	if fileCfg.modelSet {
		cfg.Model = fileCfg.model
	}
	if fileCfg.languageSet {
		cfg.Language = fileCfg.language
	}
	if fileCfg.outputFormatsSet {
		cfg.OutputFormats = cloneOutputFormats(fileCfg.outputFormats)
	}
	if fileCfg.keepIntermediateFilesSet {
		cfg.KeepIntermediateFiles = fileCfg.keepIntermediateFiles
	}
	if fileCfg.modelDirSet {
		cfg.ModelDir = fileCfg.modelDir
	}
	if fileCfg.jobsDirSet {
		cfg.JobsDir = fileCfg.jobsDir
	}
}

func applyOverrides(cfg *Config, overrides Overrides) {
	if overrides.Model != nil {
		cfg.Model = *overrides.Model
	}
	if overrides.Language != nil {
		cfg.Language = *overrides.Language
	}
	if overrides.OutputFormats != nil {
		cfg.OutputFormats = cloneOutputFormats(*overrides.OutputFormats)
	}
	if overrides.KeepIntermediateFiles != nil {
		cfg.KeepIntermediateFiles = *overrides.KeepIntermediateFiles
	}
	if overrides.ModelDir != nil {
		cfg.ModelDir = *overrides.ModelDir
	}
	if overrides.JobsDir != nil {
		cfg.JobsDir = *overrides.JobsDir
	}
}

func cloneOutputFormats(formats []domain.OutputFormat) []domain.OutputFormat {
	cloned := make([]domain.OutputFormat, len(formats))
	copy(cloned, formats)
	return cloned
}
