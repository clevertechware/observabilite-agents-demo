package commands

import (
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
	"github.com/urfave/cli/v3"
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

var flagKeys = map[string]string{
	"output":    "output",
	"namespace": "kube.namespace",
	"context":   "kube.context",
}

type settings struct {
	Output string `koanf:"output"`
	O2     struct {
		URL   string `koanf:"url"`
		Org   string `koanf:"org"`
		User  string `koanf:"user"`
		Token string `koanf:"token"`
	} `koanf:"o2"`
	Shop struct {
		URL string `koanf:"url"`
	} `koanf:"shop"`
	Kube struct {
		Namespace string `koanf:"namespace"`
		Context   string `koanf:"context"`
	} `koanf:"kube"`
}

func loadConfig(cmd *cli.Command) (*koanf.Koanf, error) {
	k := koanf.New(".")
	if err := k.Load(confmap.Provider(defaults, "."), nil); err != nil {
		return nil, err
	}
	path, err := configPath()
	if err != nil {
		return nil, err
	}
	err = k.Load(file.Provider(path), toml.Parser())
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("lecture de %s : %w", path, err)
	}
	envProvider := env.Provider(".", env.Opt{Prefix: envPrefix, TransformFunc: envKey})
	if err := k.Load(envProvider, nil); err != nil {
		return nil, err
	}
	return k, k.Load(confmap.Provider(flagsSet(cmd), "."), nil)
}

func flagsSet(cmd *cli.Command) map[string]any {
	values := map[string]any{}
	for flag, key := range flagKeys {
		if cmd.IsSet(flag) {
			values[key] = cmd.String(flag)
		}
	}
	return values
}

func envKey(name, value string) (string, any) {
	key := strings.TrimPrefix(name, envPrefix)
	return strings.ToLower(strings.ReplaceAll(key, "_", ".")), value
}

func configPath() (string, error) {
	base := os.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("répertoire personnel introuvable : %w", err)
		}
		base = filepath.Join(home, ".config")
	}
	return filepath.Join(base, "ctwctl", "config.toml"), nil
}

func decodeSettings(k *koanf.Koanf) (settings, error) {
	var s settings
	if err := k.Unmarshal("", &s); err != nil {
		return settings{}, fmt.Errorf("lecture de la configuration : %w", err)
	}
	return s, nil
}

func isConfigKey(key string) bool {
	_, ok := defaults[key]
	return ok
}

func configKeys() []string {
	keys := make([]string, 0, len(defaults))
	for key := range defaults {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func saveConfigValue(key, value string) (string, error) {
	path, err := configPath()
	if err != nil {
		return "", err
	}
	k := koanf.New(".")
	err = k.Load(file.Provider(path), toml.Parser())
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return "", fmt.Errorf("lecture de %s : %w", path, err)
	}
	if err := k.Set(key, value); err != nil {
		return "", err
	}
	data, err := k.Marshal(toml.Parser())
	if err != nil {
		return "", fmt.Errorf("encodage TOML : %w", err)
	}
	return path, writePrivateFile(path, data)
}

// writePrivateFile remplace le fichier par un renommage : le mode 0600 s'applique même si un fichier plus ouvert existait.
func writePrivateFile(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("création de %s : %w", dir, err)
	}
	tmp, err := os.CreateTemp(dir, ".config-*.toml")
	if err != nil {
		return fmt.Errorf("création du fichier temporaire : %w", err)
	}
	defer os.Remove(tmp.Name())
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return fmt.Errorf("droits du fichier temporaire : %w", err)
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return fmt.Errorf("écriture de %s : %w", path, err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("écriture de %s : %w", path, err)
	}
	return os.Rename(tmp.Name(), path)
}

func maskedConfig(k *koanf.Koanf) (*koanf.Koanf, error) {
	view := k.Copy()
	if view.String("o2.token") != "" {
		if err := view.Set("o2.token", maskedLine); err != nil {
			return nil, err
		}
	}
	return view, nil
}
