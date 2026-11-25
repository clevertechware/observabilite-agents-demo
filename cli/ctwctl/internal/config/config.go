package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/knadh/koanf/parsers/toml/v2"
	"github.com/knadh/koanf/providers/confmap"
	"github.com/knadh/koanf/providers/env/v2"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/v2"
)

const (
	envPrefix  = "CTWCTL_"
	maskedLine = "********"
)

var defaults = map[string]any{
	"o2.url":         "https://o2.example.com",
	"o2.org":         "default",
	"o2.user":        "",
	"o2.token":       "",
	"shop.url":       "https://shop.example.com",
	"kube.namespace": "demo",
	"kube.context":   "",
	"output":         "text",
}

// Settings is the resolved ctwctl configuration. The json tags give the configuration keys: o2.token is O2.Token.
type Settings struct {
	Output string       `json:"output"`
	O2     O2Settings   `json:"o2"`
	Shop   ShopSettings `json:"shop"`
	Kube   KubeSettings `json:"kube"`
}

type O2Settings struct {
	URL   string `json:"url"`
	Org   string `json:"org"`
	User  string `json:"user"`
	Token string `json:"token"`
}

type ShopSettings struct {
	URL string `json:"url"`
}

type KubeSettings struct {
	Namespace string `json:"namespace"`
	Context   string `json:"context"`
}

// Environment holds what the configuration is resolved from outside its file: the environment variables and the
// lookup of the user's home directory.
type Environment struct {
	Variables   map[string]string
	UserHomeDir func() (string, error)
}

// ProcessEnvironment returns the environment of the running process.
func ProcessEnvironment() Environment {
	variables := map[string]string{}
	for _, entry := range os.Environ() {
		name, value, _ := strings.Cut(entry, "=")
		variables[name] = value
	}

	return Environment{Variables: variables, UserHomeDir: os.UserHomeDir}
}

func (e Environment) environ() []string {
	entries := make([]string, 0, len(e.Variables))
	for name, value := range e.Variables {
		entries = append(entries, name+"="+value)
	}

	return entries
}

// Load resolves the settings from, in increasing priority, the defaults, the configuration file, the CTWCTL_*
// variables of environment and flagValues.
func Load(environment Environment, flagValues map[string]any) (Settings, error) {
	k := koanf.New(".")
	if err := k.Load(confmap.Provider(defaults, "."), nil); err != nil {
		return Settings{}, err
	}

	path, err := configPath(environment)
	if err != nil {
		return Settings{}, err
	}

	err = k.Load(file.Provider(path), toml.Parser())
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return Settings{}, fmt.Errorf("read %s: %w", path, err)
	}

	envProvider := env.Provider(".", env.Opt{
		Prefix:        envPrefix,
		TransformFunc: envKey,
		EnvironFunc:   environment.environ,
	})
	if err = k.Load(envProvider, nil); err != nil {
		return Settings{}, err
	}

	if err = k.Load(confmap.Provider(flagValues, "."), nil); err != nil {
		return Settings{}, err
	}

	var settings Settings
	if err = k.UnmarshalWithConf("", &settings, koanf.UnmarshalConf{Tag: "json"}); err != nil {
		return Settings{}, fmt.Errorf("read configuration: %w", err)
	}

	return settings, nil
}

// Masked returns a copy of s whose token, when set, is replaced so that it can be displayed.
func (s Settings) Masked() Settings {
	if s.O2.Token != "" {
		s.O2.Token = maskedLine
	}

	return s
}

// ByKey returns the settings as a map from each configuration key, for example o2.token, to its value.
func (s Settings) ByKey() (map[string]any, error) {
	data, err := json.Marshal(s)
	if err != nil {
		return nil, err
	}

	var nested map[string]any
	if err = json.Unmarshal(data, &nested); err != nil {
		return nil, err
	}

	k := koanf.New(".")
	if err = k.Load(confmap.Provider(nested, "."), nil); err != nil {
		return nil, err
	}

	return k.All(), nil
}

func IsKey(key string) bool {
	_, ok := defaults[key]
	return ok
}

func Keys() []string {
	keys := make([]string, 0, len(defaults))
	for key := range defaults {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func SaveValue(environment Environment, key, value string) (string, error) {
	path, err := configPath(environment)
	if err != nil {
		return "", err
	}

	k := koanf.New(".")
	err = k.Load(file.Provider(path), toml.Parser())
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return "", fmt.Errorf("read %s: %w", path, err)
	}

	if err = k.Set(key, value); err != nil {
		return "", err
	}

	data, err := k.Marshal(toml.Parser())
	if err != nil {
		return "", fmt.Errorf("encode TOML: %w", err)
	}

	return path, writePrivateFile(path, data)
}

// configPath follows the XDG Base Directory specification and returns the path of the configuration file.
//
// XDG_CONFIG_HOME names the directory of the user's configuration files, ~/.config by default.
//
// This directory is also used on macOS, as gh and git do, rather than the "~/Library/Application Support" returned
// by os.UserConfigDir.
func configPath(environment Environment) (string, error) {
	base := environment.Variables["XDG_CONFIG_HOME"]
	if base == "" {
		home, err := environment.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("home directory not found: %w", err)
		}
		base = filepath.Join(home, ".config")
	}

	return filepath.Join(base, "ctwctl", "config.toml"), nil
}

func envKey(name, value string) (string, any) {
	key := strings.TrimPrefix(name, envPrefix)
	return strings.ToLower(strings.ReplaceAll(key, "_", ".")), value
}

func writePrivateFile(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create %s: %w", dir, err)
	}

	tmp, err := os.CreateTemp(dir, ".config-*.toml")
	if err != nil {
		return fmt.Errorf("create temporary file: %w", err)
	}
	defer func() { _ = os.Remove(tmp.Name()) }()

	if err = tmp.Chmod(0o600); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("set temporary file permissions: %w", err)
	}

	if _, err = tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write %s: %w", path, err)
	}

	if err = tmp.Close(); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}

	return os.Rename(tmp.Name(), path)
}
