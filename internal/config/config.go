package config

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/BurntSushi/toml"
	"gopkg.in/yaml.v3"
)

type Registration struct {
	ID    string `yaml:"id"    toml:"id"    mapstructure:"id"`
	Token string `yaml:"token" toml:"token" mapstructure:"token"`
	URL   string `yaml:"url"   toml:"url"   mapstructure:"url"`
}

type Config struct {
	ID            string         `yaml:"id"             toml:"id"             mapstructure:"id"`
	Token         string         `yaml:"token"          toml:"token"          mapstructure:"token"`
	URL           string         `yaml:"url"            toml:"url"            mapstructure:"url"`
	Passkey       string         `yaml:"passkey"        toml:"passkey"        mapstructure:"passkey"`
	Tags          []string       `yaml:"tags"           toml:"tags"           mapstructure:"tags"`
	Origin        string         `yaml:"origin"         toml:"origin"         mapstructure:"origin"`
	Server        string         `yaml:"server"         toml:"server"         mapstructure:"server"`
	ServerMID     string         `yaml:"server_mid"     toml:"server_mid"     mapstructure:"server_mid"`
	NoServer      bool           `yaml:"no_server"      toml:"no_server"      mapstructure:"no_server"`
	User          string         `yaml:"user"           toml:"user"           mapstructure:"user"`
	Name          string         `yaml:"name"           toml:"name"           mapstructure:"name"`
	Verbose       int            `yaml:"verbose"        toml:"verbose"        mapstructure:"verbose"`
	Daemon        bool           `yaml:"daemon"         toml:"daemon"         mapstructure:"daemon"`
	Logfile       string         `yaml:"logfile"        toml:"logfile"        mapstructure:"logfile"`
	Pidfile       string         `yaml:"pidfile"        toml:"pidfile"        mapstructure:"pidfile"`
	ChunkSize     int            `yaml:"chunk_size"     toml:"chunk_size"     mapstructure:"chunk_size"`
	AllowVerbs    []string       `yaml:"allow_verbs"    toml:"allow_verbs"    mapstructure:"allow_verbs"`
	DenyVerbs     []string       `yaml:"deny_verbs"     toml:"deny_verbs"     mapstructure:"deny_verbs"`
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

// claude: SetSaveFormat overrides the save file format and adjusts the
// file path extension accordingly. Used by --save-format flag.
func (c *Config) SetSaveFormat(format string) {
	c.fileFormat = format
	if c.filePath != "" {
		ext := filepath.Ext(c.filePath)
		base := strings.TrimSuffix(c.filePath, ext)
		switch format {
		case "toml":
			c.filePath = base + ".toml"
		case "yaml":
			c.filePath = base + ".yml"
		}
	}
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

	// claude: check if list fields are strings before strict unmarshal
	strFields := map[string]string{}
	needRemarshal := false
	for _, field := range []string{"tags", "allow_verbs", "deny_verbs"} {
		if v, ok := raw[field]; ok {
			if s, ok := v.(string); ok {
				strFields[field] = s
				delete(raw, field)
				needRemarshal = true
			}
		}
	}
	if needRemarshal {
		data, _ = yaml.Marshal(raw)
	}

	cfg := &Config{fileFormat: format}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, err
	}

	if s, ok := strFields["tags"]; ok {
		cfg.Tags = splitTags(s)
	}
	if s, ok := strFields["allow_verbs"]; ok {
		cfg.AllowVerbs = splitTags(s)
	}
	if s, ok := strFields["deny_verbs"]; ok {
		cfg.DenyVerbs = splitTags(s)
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

// AllControlledVerbs lists the verbs that can be allowed/denied.
var AllControlledVerbs = []string{"exec", "eval", "get_file", "put_file", "file_exists"}

// ResolveAllowedVerbs computes the final set of allowed verbs.
// If AllowVerbs is empty, all controlled verbs are allowed.
// DenyVerbs are then removed from the allowed set.
func (c *Config) ResolveAllowedVerbs() map[string]bool {
	allowed := make(map[string]bool)

	if len(c.AllowVerbs) == 0 {
		for _, v := range AllControlledVerbs {
			allowed[v] = true
		}
	} else {
		for _, v := range c.AllowVerbs {
			v = strings.TrimSpace(v)
			if v != "" {
				allowed[v] = true
			}
		}
	}

	for _, v := range c.DenyVerbs {
		v = strings.TrimSpace(v)
		delete(allowed, v)
	}

	return allowed
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
				if c.URL == "" && r.URL != "" {
					c.URL = r.URL
				}
				return
			}
		}
	} else if c.ID == "" && len(regs) == 1 {
		c.ID = regs[0].ID
		c.Token = regs[0].Token
		if c.URL == "" && regs[0].URL != "" {
			c.URL = regs[0].URL
		}
	}
}

func (c *Config) Save(data map[string]interface{}) error {
	path := c.filePath
	if path == "" {
		cwd, _ := os.Getwd()
		path = filepath.Join(cwd, "cla-worker.toml")
		c.filePath = path
		c.fileFormat = "toml"
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
					url, _ := m["url"].(string)
					if id != "" {
						regMap[id] = Registration{ID: id, Token: token, URL: url}
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
		entry := map[string]interface{}{
			"id":    r.ID,
			"token": r.Token,
		}
		if r.URL != "" {
			entry["url"] = r.URL
		}
		result = append(result, entry)
	}
	return result
}

// claude: SystemConfigDir returns the system-level configuration directory.
func SystemConfigDir() string {
	if runtime.GOOS == "windows" {
		if pd := os.Getenv("ProgramData"); pd != "" {
			return filepath.Join(pd, "cla-worker")
		}
		return `C:\ProgramData\cla-worker`
	}
	return "/etc"
}

// claude: SystemConfigPath returns the path to the system-level config file.
func SystemConfigPath() string {
	return filepath.Join(SystemConfigDir(), "cla-worker.toml")
}

// claude: InstallSystemConfig copies the current configuration to the system
// config location, setting top-level id and url fields for service startup.
// Returns the path to the installed system config file.
func (c *Config) InstallSystemConfig() (string, error) {
	destPath := SystemConfigPath()

	dir := filepath.Dir(destPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("creating config directory %s: %w", dir, err)
	}

	existing := make(map[string]interface{})
	if c.filePath != "" {
		data, err := os.ReadFile(c.filePath)
		if err != nil {
			return "", fmt.Errorf("reading source config %s: %w", c.filePath, err)
		}
		ext := strings.ToLower(filepath.Ext(c.filePath))
		switch ext {
		case ".toml":
			if err := toml.Unmarshal(data, &existing); err != nil {
				return "", fmt.Errorf("parsing source config: %w", err)
			}
		default:
			if err := yaml.Unmarshal(data, &existing); err != nil {
				return "", fmt.Errorf("parsing source config: %w", err)
			}
		}
	}

	if c.ID != "" {
		existing["id"] = c.ID
	}
	if c.URL != "" {
		existing["url"] = c.URL
	}

	buf := &strings.Builder{}
	enc := toml.NewEncoder(buf)
	if err := enc.Encode(existing); err != nil {
		return "", fmt.Errorf("encoding config: %w", err)
	}

	if err := os.WriteFile(destPath, []byte(buf.String()), 0644); err != nil {
		return "", fmt.Errorf("writing system config %s: %w", destPath, err)
	}

	return destPath, nil
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
