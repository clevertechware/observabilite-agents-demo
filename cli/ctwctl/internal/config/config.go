package config

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

type Settings struct {
	Output string       `koanf:"output"`
	O2     O2Settings   `koanf:"o2"`
	Shop   ShopSettings `koanf:"shop"`
	Kube   KubeSettings `koanf:"kube"`
}

type O2Settings struct {
	URL   string `koanf:"url"`
	Org   string `koanf:"org"`
	User  string `koanf:"user"`
	Token string `koanf:"token"`
}

type ShopSettings struct {
	URL string `koanf:"url"`
}

type KubeSettings struct {
	Namespace string `koanf:"namespace"`
	Context   string `koanf:"context"`
}

type Resolved struct {
	Settings Settings
	values   *koanf.Koanf
}

type View struct {
	Raw    map[string]any
	Values map[string]any
}

func Load(flagValues map[string]any) (*Resolved, error) {
	k := koanf.New(".")
	if err := k.Load(confmap.Provider(defaults, "."), nil); err != nil {
		return nil, err
	}

	path, err := configPath(os.UserHomeDir)
	if err != nil {
		return nil, err
	}

	err = k.Load(file.Provider(path), toml.Parser())
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("lecture de %s : %w", path, err)
	}

	envProvider := env.Provider(".", env.Opt{Prefix: envPrefix, TransformFunc: envKey})
	if err = k.Load(envProvider, nil); err != nil {
		return nil, err
	}

	if err = k.Load(confmap.Provider(flagValues, "."), nil); err != nil {
		return nil, err
	}

	var settings Settings
	if err = k.Unmarshal("", &settings); err != nil {
		return nil, fmt.Errorf("lecture de la configuration : %w", err)
	}

	return &Resolved{Settings: settings, values: k}, nil
}

func (r *Resolved) MaskedView() (View, error) {
	view := r.values.Copy()
	if view.String("o2.token") != "" {
		if err := view.Set("o2.token", maskedLine); err != nil {
			return View{}, err
		}
	}

	return View{Raw: view.Raw(), Values: view.All()}, nil
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

func SaveValue(key, value string) (string, error) {
	path, err := configPath(os.UserHomeDir)
	if err != nil {
		return "", err
	}

	k := koanf.New(".")
	err = k.Load(file.Provider(path), toml.Parser())
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return "", fmt.Errorf("lecture de %s : %w", path, err)
	}

	if err = k.Set(key, value); err != nil {
		return "", err
	}

	data, err := k.Marshal(toml.Parser())
	if err != nil {
		return "", fmt.Errorf("encodage TOML : %w", err)
	}

	return path, writePrivateFile(path, data)
}

// configPath suit la spécification XDG Base Directory et retourne le chemin du fichier de configuration.
//
// XDG_CONFIG_HOME désigne le répertoire des fichiers de configuration de l'utilisateur avec ~/.config par défaut.
//
// Ce répertoire est retenu aussi sous macOS, comme le font gh ou git, plutôt que le "~/Library/Application Support"
// renvoyé par os.UserConfigDir.
func configPath(userHomeDir func() (string, error)) (string, error) {
	base := os.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		home, err := userHomeDir()
		if err != nil {
			return "", fmt.Errorf("répertoire personnel introuvable : %w", err)
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
		return fmt.Errorf("création de %s : %w", dir, err)
	}

	tmp, err := os.CreateTemp(dir, ".config-*.toml")
	if err != nil {
		return fmt.Errorf("création du fichier temporaire : %w", err)
	}
	defer os.Remove(tmp.Name())

	if err = tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return fmt.Errorf("droits du fichier temporaire : %w", err)
	}

	if _, err = tmp.Write(data); err != nil {
		tmp.Close()
		return fmt.Errorf("écriture de %s : %w", path, err)
	}

	if err = tmp.Close(); err != nil {
		return fmt.Errorf("écriture de %s : %w", path, err)
	}

	return os.Rename(tmp.Name(), path)
}
