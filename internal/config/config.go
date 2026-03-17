package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
	"gopkg.in/yaml.v3"
)

type Registration struct {
	ID    string `yaml:"id"    toml:"id"    mapstructure:"id"`
	Token string `yaml:"token" toml:"token" mapstructure:"token"`
}

type Config struct {
	ID            string         `yaml:"id"             toml:"id"             mapstructure:"id"`
	Token         string         `yaml:"token"          toml:"token"          mapstructure:"token"`
	URL           string         `yaml:"url"            toml:"url"            mapstructure:"url"`
	Passkey       string         `yaml:"passkey"        toml:"passkey"        mapstructure:"passkey"`
	Tags          []string       `yaml:"tags"           toml:"tags"           mapstructure:"tags"`
	Origin        string         `yaml:"origin"         toml:"origin"         mapstructure:"origin"`
	Verbose       int            `yaml:"verbose"        toml:"verbose"        mapstructure:"verbose"`
	Daemon        bool           `yaml:"daemon"         toml:"daemon"         mapstructure:"daemon"`
	Logfile       string         `yaml:"logfile"        toml:"logfile"        mapstructure:"logfile"`
	Pidfile       string         `yaml:"pidfile"        toml:"pidfile"        mapstructure:"pidfile"`
	ChunkSize     int            `yaml:"chunk_size"     toml:"chunk_size"     mapstructure:"chunk_size"`
	Registrations []Registration `yaml:"registrations"  toml:"registrations"  mapstructure:"registrations"`

	filePath   string
	fileFormat string
}

func (c *Config) FilePath() string {
	return c.filePath
}

func (c *Config) FileFormat() string {
	return c.fileFormat
}

func Load(explicit string) (*Config, error) {
	candidates := searchPaths(explicit)

	for _, path := range candidates {
		if path == "" {
			continue
		}

		data, err := os.ReadFile(path)
		if err != nil {
			if os.IsNotExist(err) {
				if path == explicit {
					return nil, fmt.Errorf("config file not found: %s", path)
				}
				continue
			}
			return nil, fmt.Errorf("reading config %s: %w", path, err)
		}

		cfg, err := parse(path, data)
		if err != nil {
			return nil, fmt.Errorf("parsing config %s: %w", path, err)
		}

		cfg.filePath = path
		cfg.setDefaults()
		return cfg, nil
	}

	cfg := &Config{}
	cfg.setDefaults()
	return cfg, nil
}

func (c *Config) setDefaults() {
	if c.URL == "" {
		c.URL = "http://localhost:8080"
	}
	if c.ChunkSize == 0 {
		c.ChunkSize = 64 * 1024
	}
	cwd, _ := os.Getwd()
	if c.Logfile == "" {
		c.Logfile = filepath.Join(cwd, "cla-worker.log")
	}
	if c.Pidfile == "" {
		c.Pidfile = filepath.Join(cwd, "cla-worker.pid")
	}
}

func searchPaths(explicit string) []string {
	if explicit != "" {
		return []string{explicit}
	}

	var paths []string

	if envPath := os.Getenv("CLA_WORKER_CONFIG"); envPath != "" {
		paths = append(paths, envPath)
	}

	cwd, _ := os.Getwd()
	paths = append(paths,
		filepath.Join(cwd, "cla-worker.yml"),
		filepath.Join(cwd, "cla-worker.toml"),
	)

	if home, err := os.UserHomeDir(); err == nil {
		paths = append(paths,
			filepath.Join(home, "cla-worker.yml"),
			filepath.Join(home, "cla-worker.toml"),
		)
	}

	paths = append(paths,
		"/etc/cla-worker.yml",
		"/etc/cla-worker.toml",
	)

	return paths
}

func parse(path string, data []byte) (*Config, error) {
	ext := strings.ToLower(filepath.Ext(path))

	switch ext {
	case ".yml", ".yaml":
		return parseYAML(data, "yaml")
	case ".toml":
		return parseTOML(data)
	default:
		cfg, err := parseYAML(data, "yaml")
		if err == nil {
			return cfg, nil
		}
		return parseTOML(data)
	}
}

func parseYAML(data []byte, format string) (*Config, error) {
	var raw map[string]interface{}
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, err
	}

	// Check if tags is a string before strict unmarshal
	var tagsStr string
	isTagsString := false
	if tags, ok := raw["tags"]; ok {
		if s, ok := tags.(string); ok {
			isTagsString = true
			tagsStr = s
			delete(raw, "tags")
			// Re-marshal without the tags string for struct unmarshal
			data, _ = yaml.Marshal(raw)
		}
	}

	cfg := &Config{fileFormat: format}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, err
	}

	if isTagsString {
		cfg.Tags = splitTags(tagsStr)
	}

	return cfg, nil
}

func parseTOML(data []byte) (*Config, error) {
	cfg := &Config{fileFormat: "toml"}
	if err := toml.Unmarshal(data, cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

func splitTags(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}

func (c *Config) ResolveTags() {
	if c.Tags == nil {
		c.Tags = []string{}
	}
}

func (c *Config) ResolveToken() {
	regs := c.Registrations
	if len(regs) == 0 {
		return
	}

	if c.ID != "" && c.Token == "" {
		for _, r := range regs {
			if r.ID == c.ID {
				c.Token = r.Token
				return
			}
		}
	} else if c.ID == "" && len(regs) == 1 {
		c.ID = regs[0].ID
		c.Token = regs[0].Token
	}
}

func (c *Config) Save(data map[string]interface{}) error {
	path := c.filePath
	if path == "" {
		cwd, _ := os.Getwd()
		path = filepath.Join(cwd, "cla-worker.yml")
		c.filePath = path
		c.fileFormat = "yaml"
	}

	existing, err := loadExisting(path)
	if err != nil {
		existing = make(map[string]interface{})
	}

	for k, v := range data {
		if k == "registrations" {
			if newRegs, ok := v.([]Registration); ok {
				existing["registrations"] = mergeRegistrations(existing, newRegs)
				continue
			}
		}
		existing[k] = v
	}

	var out []byte
	format := detectFormat(path, c.fileFormat)
	switch format {
	case "toml":
		buf := &strings.Builder{}
		enc := toml.NewEncoder(buf)
		if err := enc.Encode(existing); err != nil {
			return fmt.Errorf("encoding TOML: %w", err)
		}
		out = []byte(buf.String())
	default:
		out, err = yaml.Marshal(existing)
		if err != nil {
			return fmt.Errorf("encoding YAML: %w", err)
		}
	}

	if err := os.WriteFile(path, out, 0644); err != nil {
		return fmt.Errorf("writing config %s: %w", path, err)
	}
	return nil
}

func loadExisting(path string) (map[string]interface{}, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	ext := strings.ToLower(filepath.Ext(path))
	result := make(map[string]interface{})

	switch ext {
	case ".toml":
		if err := toml.Unmarshal(data, &result); err != nil {
			return nil, err
		}
	default:
		if err := yaml.Unmarshal(data, &result); err != nil {
			return nil, err
		}
	}
	return result, nil
}

func mergeRegistrations(existing map[string]interface{}, newRegs []Registration) []interface{} {
	regMap := make(map[string]Registration)

	if existRegs, ok := existing["registrations"]; ok {
		switch regs := existRegs.(type) {
		case []interface{}:
			for _, r := range regs {
				if m, ok := r.(map[string]interface{}); ok {
					id, _ := m["id"].(string)
					token, _ := m["token"].(string)
					if id != "" {
						regMap[id] = Registration{ID: id, Token: token}
					}
				}
			}
		}
	}

	for _, r := range newRegs {
		regMap[r.ID] = r
	}

	result := make([]interface{}, 0, len(regMap))
	for _, r := range regMap {
		result = append(result, map[string]interface{}{
			"id":    r.ID,
			"token": r.Token,
		})
	}
	return result
}

func detectFormat(path, fallback string) string {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".toml":
		return "toml"
	case ".yml", ".yaml":
		return "yaml"
	}
	if fallback != "" {
		return fallback
	}
	return "yaml"
}
