package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testdataPath(name string) string {
	return filepath.Join("..", "..", "tests", "testdata", name)
}

func TestLoad_ExplicitYAML(t *testing.T) {
	cfg, err := Load(testdataPath("valid_config.yml"))
	require.NoError(t, err)
	assert.Equal(t, "worker-1", cfg.ID)
	assert.Equal(t, "tok-abc123", cfg.Token)
	assert.Equal(t, "http://example.com:8080", cfg.URL)
	assert.Equal(t, []string{"linux", "docker"}, cfg.Tags)
	assert.Equal(t, 1, cfg.Verbose)
	assert.Equal(t, 65536, cfg.ChunkSize)
	assert.Len(t, cfg.Registrations, 2)
}

func TestLoad_ExplicitTOML(t *testing.T) {
	cfg, err := Load(testdataPath("valid_config.toml"))
	require.NoError(t, err)
	assert.Equal(t, "worker-1", cfg.ID)
	assert.Equal(t, "tok-abc123", cfg.Token)
	assert.Equal(t, "http://example.com:8080", cfg.URL)
	assert.Equal(t, []string{"linux", "docker"}, cfg.Tags)
	assert.Len(t, cfg.Registrations, 2)
}

func TestLoad_Minimal(t *testing.T) {
	cfg, err := Load(testdataPath("minimal_config.yml"))
	require.NoError(t, err)
	assert.Equal(t, "http://minimal.example.com", cfg.URL)
	assert.NotZero(t, cfg.ChunkSize, "default chunk_size should be set")
}

func TestLoad_Malformed(t *testing.T) {
	_, err := Load(testdataPath("malformed_config.yml"))
	require.Error(t, err)
}

func TestLoad_FileNotFound(t *testing.T) {
	_, err := Load("/nonexistent/path/config.yml")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestLoad_NoFileReturnsDefaults(t *testing.T) {
	cfg, err := Load("")
	require.NoError(t, err)
	// claude: URL default moved to root.go PersistentPreRunE
	assert.Empty(t, cfg.URL)
	assert.Equal(t, 64*1024, cfg.ChunkSize)
}

func TestLoad_EnvVar(t *testing.T) {
	path := testdataPath("valid_config.yml")
	abs, _ := filepath.Abs(path)
	t.Setenv("CLA_WORKER_CONFIG", abs)
	cfg, err := Load("")
	require.NoError(t, err)
	assert.Equal(t, "worker-1", cfg.ID)
}

func TestConfig_ResolveTags_Nil(t *testing.T) {
	cfg := &Config{}
	cfg.ResolveTags()
	assert.NotNil(t, cfg.Tags)
	assert.Empty(t, cfg.Tags)
}

func TestConfig_ResolveTags_Existing(t *testing.T) {
	cfg := &Config{Tags: []string{"a", "b"}}
	cfg.ResolveTags()
	assert.Equal(t, []string{"a", "b"}, cfg.Tags)
}

func TestConfig_ResolveToken_IDMatchesRegistration(t *testing.T) {
	cfg, err := Load(testdataPath("multi_registration.yml"))
	require.NoError(t, err)
	cfg.Token = ""
	cfg.URL = ""
	cfg.ResolveToken()
	assert.Equal(t, "token-2", cfg.Token)
	assert.Equal(t, "http://host2:8080", cfg.URL)
}

func TestConfig_ResolveToken_NoMatch(t *testing.T) {
	cfg := &Config{
		ID: "unknown",
		Registrations: []Registration{
			{ID: "worker-1", Token: "token-1"},
		},
	}
	cfg.ResolveToken()
	assert.Empty(t, cfg.Token)
}

func TestConfig_ResolveToken_SingleRegistration(t *testing.T) {
	cfg := &Config{
		Registrations: []Registration{
			{ID: "only-one", Token: "only-token", URL: "http://myserver:8080"},
		},
	}
	cfg.ResolveToken()
	assert.Equal(t, "only-one", cfg.ID)
	assert.Equal(t, "only-token", cfg.Token)
	assert.Equal(t, "http://myserver:8080", cfg.URL)
}

func TestConfig_ResolveToken_NoRegistrations(t *testing.T) {
	cfg := &Config{ID: "test"}
	cfg.ResolveToken()
	assert.Empty(t, cfg.Token)
}

func TestConfig_ResolveToken_URLNotOverriddenIfSet(t *testing.T) {
	cfg := &Config{
		URL: "http://explicit:9999",
		Registrations: []Registration{
			{ID: "only-one", Token: "only-token", URL: "http://reg:8080"},
		},
	}
	cfg.ResolveToken()
	assert.Equal(t, "only-one", cfg.ID)
	assert.Equal(t, "only-token", cfg.Token)
	// claude: explicit URL should not be overridden by registration URL
	assert.Equal(t, "http://explicit:9999", cfg.URL)
}

func TestConfig_ResolveToken_URLFromIDMatch(t *testing.T) {
	cfg := &Config{
		ID: "w1",
		Registrations: []Registration{
			{ID: "w1", Token: "t1", URL: "http://matched:8080"},
			{ID: "w2", Token: "t2", URL: "http://other:8080"},
		},
	}
	cfg.ResolveToken()
	assert.Equal(t, "t1", cfg.Token)
	assert.Equal(t, "http://matched:8080", cfg.URL)
}

func TestConfig_ResolveToken_MultipleRegistrationsNoID(t *testing.T) {
	cfg := &Config{
		Registrations: []Registration{
			{ID: "a", Token: "ta"},
			{ID: "b", Token: "tb"},
		},
	}
	cfg.ResolveToken()
	assert.Empty(t, cfg.ID)
	assert.Empty(t, cfg.Token)
}

func TestSplitTags_Comma(t *testing.T) {
	tags := splitTags("linux, docker , go")
	assert.Equal(t, []string{"linux", "docker", "go"}, tags)
}

func TestSplitTags_Empty(t *testing.T) {
	tags := splitTags("")
	assert.Nil(t, tags)
}

func TestYAML_TagsAsString(t *testing.T) {
	tmpDir := t.TempDir()
	f := filepath.Join(tmpDir, "tags_string.yml")
	err := os.WriteFile(f, []byte("tags: linux,docker,go\n"), 0644)
	require.NoError(t, err)

	cfg, err := Load(f)
	require.NoError(t, err)
	assert.Equal(t, []string{"linux", "docker", "go"}, cfg.Tags)
}

func TestConfig_Save_YAML(t *testing.T) {
	tmpDir := t.TempDir()
	f := filepath.Join(tmpDir, "save_test.yml")
	err := os.WriteFile(f, []byte("id: original\n"), 0644)
	require.NoError(t, err)

	cfg, err := Load(f)
	require.NoError(t, err)

	err = cfg.Save(map[string]interface{}{
		"id": "updated",
	})
	require.NoError(t, err)

	cfg2, err := Load(f)
	require.NoError(t, err)
	assert.Equal(t, "updated", cfg2.ID)
}

func TestConfig_Save_TOML(t *testing.T) {
	tmpDir := t.TempDir()
	f := filepath.Join(tmpDir, "save_test.toml")
	err := os.WriteFile(f, []byte("id = \"original\"\n"), 0644)
	require.NoError(t, err)

	cfg, err := Load(f)
	require.NoError(t, err)

	err = cfg.Save(map[string]interface{}{
		"id": "updated-toml",
	})
	require.NoError(t, err)

	cfg2, err := Load(f)
	require.NoError(t, err)
	assert.Equal(t, "updated-toml", cfg2.ID)
}

func TestConfig_Save_MergeRegistrations(t *testing.T) {
	tmpDir := t.TempDir()
	f := filepath.Join(tmpDir, "merge_reg.yml")
	initial := "registrations:\n  - id: w1\n    token: t1\n"
	err := os.WriteFile(f, []byte(initial), 0644)
	require.NoError(t, err)

	cfg, err := Load(f)
	require.NoError(t, err)

	err = cfg.Save(map[string]interface{}{
		"registrations": []Registration{{ID: "w2", Token: "t2"}},
	})
	require.NoError(t, err)

	cfg2, err := Load(f)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(cfg2.Registrations), 2)
}

func TestConfig_Save_NewFile(t *testing.T) {
	tmpDir := t.TempDir()
	oldCwd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(oldCwd)

	cfg := &Config{}
	err := cfg.Save(map[string]interface{}{
		"id": "new-worker",
	})
	require.NoError(t, err)

	// claude: default save format is now TOML
	f := filepath.Join(tmpDir, "cla-worker.toml")
	_, err = os.Stat(f)
	require.NoError(t, err)
}

func TestConfig_Defaults(t *testing.T) {
	cfg := &Config{}
	cfg.setDefaults()
	// claude: URL default moved to root.go PersistentPreRunE
	assert.Empty(t, cfg.URL)
	assert.Equal(t, 64*1024, cfg.ChunkSize)
	assert.NotEmpty(t, cfg.Logfile)
	assert.NotEmpty(t, cfg.Pidfile)
}

func TestDetectFormat_YAML(t *testing.T) {
	assert.Equal(t, "yaml", detectFormat("foo.yml", ""))
	assert.Equal(t, "yaml", detectFormat("foo.yaml", ""))
}

func TestDetectFormat_TOML(t *testing.T) {
	assert.Equal(t, "toml", detectFormat("foo.toml", ""))
}

func TestDetectFormat_Fallback(t *testing.T) {
	assert.Equal(t, "toml", detectFormat("foo.conf", "toml"))
	assert.Equal(t, "yaml", detectFormat("foo.conf", ""))
}

// claude: verb control config tests

func TestLoad_DenyVerbsYAML(t *testing.T) {
	cfg, err := Load(testdataPath("deny_exec_config.yml"))
	require.NoError(t, err)
	assert.Equal(t, []string{"exec"}, cfg.DenyVerbs)
	assert.Nil(t, cfg.AllowVerbs)
}

func TestLoad_AllowVerbsOnlyYAML(t *testing.T) {
	cfg, err := Load(testdataPath("allow_exec_only_config.yml"))
	require.NoError(t, err)
	assert.Equal(t, []string{"exec"}, cfg.AllowVerbs)
	assert.Nil(t, cfg.DenyVerbs)
}

func TestLoad_DenyVerbsTOML(t *testing.T) {
	cfg, err := Load(testdataPath("deny_verbs_config.toml"))
	require.NoError(t, err)
	assert.Equal(t, []string{"exec", "put_file"}, cfg.DenyVerbs)
}

func TestLoad_AllowVerbsAsString(t *testing.T) {
	cfg, err := Load(testdataPath("allow_verbs_string_config.yml"))
	require.NoError(t, err)
	assert.Equal(t, []string{"exec", "get_file"}, cfg.AllowVerbs)
}

func TestResolveAllowedVerbs_DefaultAllowsAll(t *testing.T) {
	cfg := &Config{}
	allowed := cfg.ResolveAllowedVerbs()
	for _, v := range AllControlledVerbs {
		assert.True(t, allowed[v], "verb %s should be allowed by default", v)
	}
}

func TestResolveAllowedVerbs_AllowOnly(t *testing.T) {
	cfg := &Config{AllowVerbs: []string{"exec"}}
	allowed := cfg.ResolveAllowedVerbs()
	assert.True(t, allowed["exec"])
	assert.False(t, allowed["get_file"])
	assert.False(t, allowed["put_file"])
	assert.False(t, allowed["eval"])
	assert.False(t, allowed["file_exists"])
}

func TestResolveAllowedVerbs_DenyOnly(t *testing.T) {
	cfg := &Config{DenyVerbs: []string{"exec", "put_file"}}
	allowed := cfg.ResolveAllowedVerbs()
	assert.False(t, allowed["exec"])
	assert.False(t, allowed["put_file"])
	assert.True(t, allowed["get_file"])
	assert.True(t, allowed["eval"])
	assert.True(t, allowed["file_exists"])
}

func TestResolveAllowedVerbs_DenyOverridesAllow(t *testing.T) {
	cfg := &Config{
		AllowVerbs: []string{"exec", "get_file"},
		DenyVerbs:  []string{"exec"},
	}
	allowed := cfg.ResolveAllowedVerbs()
	assert.False(t, allowed["exec"], "deny should override allow")
	assert.True(t, allowed["get_file"])
	assert.False(t, allowed["put_file"])
}

func TestResolveAllowedVerbs_EmptyDenyNoEffect(t *testing.T) {
	cfg := &Config{DenyVerbs: []string{}}
	allowed := cfg.ResolveAllowedVerbs()
	for _, v := range AllControlledVerbs {
		assert.True(t, allowed[v])
	}
}

func TestConfig_Save_MergeRegistrationsWithURL(t *testing.T) {
	tmpDir := t.TempDir()
	f := filepath.Join(tmpDir, "merge_url.yml")
	initial := "registrations:\n  - id: w1\n    token: t1\n    url: http://old:8080\n"
	err := os.WriteFile(f, []byte(initial), 0644)
	require.NoError(t, err)

	cfg, err := Load(f)
	require.NoError(t, err)

	err = cfg.Save(map[string]interface{}{
		"registrations": []Registration{{ID: "w2", Token: "t2", URL: "http://new:9090"}},
	})
	require.NoError(t, err)

	cfg2, err := Load(f)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(cfg2.Registrations), 2)

	// claude: verify URL is preserved in merged registrations
	urlMap := map[string]string{}
	for _, r := range cfg2.Registrations {
		urlMap[r.ID] = r.URL
	}
	assert.Equal(t, "http://old:8080", urlMap["w1"])
	assert.Equal(t, "http://new:9090", urlMap["w2"])
}

func TestConfig_SetSaveFormat(t *testing.T) {
	cfg := &Config{filePath: "/tmp/cla-worker.yml", fileFormat: "yaml"}
	cfg.SetSaveFormat("toml")
	assert.Equal(t, "toml", cfg.FileFormat())
	assert.Equal(t, "/tmp/cla-worker.toml", cfg.FilePath())

	cfg.SetSaveFormat("yaml")
	assert.Equal(t, "yaml", cfg.FileFormat())
	assert.Equal(t, "/tmp/cla-worker.yml", cfg.FilePath())
}

func TestConfig_MaxLogSize_Default(t *testing.T) {
	cfg := &Config{}
	cfg.setDefaults()
	assert.Equal(t, 20, cfg.MaxLogSize)
}

func TestRotateLogfile_EmptyPath(t *testing.T) {
	cfg := &Config{Logfile: "", MaxLogSize: 20}
	err := cfg.RotateLogfile()
	assert.NoError(t, err)
}

func TestRotateLogfile_MissingFile(t *testing.T) {
	cfg := &Config{Logfile: "/nonexistent/path/foo.log", MaxLogSize: 20}
	err := cfg.RotateLogfile()
	assert.NoError(t, err)
}

func TestRotateLogfile_ZeroMaxSize(t *testing.T) {
	cfg := &Config{Logfile: "/tmp/foo.log", MaxLogSize: 0}
	err := cfg.RotateLogfile()
	assert.NoError(t, err)
}

func TestRotateLogfile_UnderLimit(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "test.log")
	err := os.WriteFile(logPath, []byte("small"), 0644)
	require.NoError(t, err)

	cfg := &Config{Logfile: logPath, MaxLogSize: 1}
	err = cfg.RotateLogfile()
	assert.NoError(t, err)

	// claude: original file should still exist, no backup created
	_, err = os.Stat(logPath)
	assert.NoError(t, err)
	_, err = os.Stat(logPath + ".1")
	assert.True(t, os.IsNotExist(err))
}

func TestRotateLogfile_OverLimit(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "test.log")

	// claude: create a file just over 1 MB
	data := make([]byte, 1*1024*1024+1)
	err := os.WriteFile(logPath, data, 0644)
	require.NoError(t, err)

	cfg := &Config{Logfile: logPath, MaxLogSize: 1}
	err = cfg.RotateLogfile()
	assert.NoError(t, err)

	// claude: original should be gone, backup should exist
	_, err = os.Stat(logPath)
	assert.True(t, os.IsNotExist(err))
	info, err := os.Stat(logPath + ".1")
	assert.NoError(t, err)
	assert.Equal(t, int64(1*1024*1024+1), info.Size())
}

func TestRotateLogfile_OverwritesExistingBackup(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "test.log")
	backupPath := logPath + ".1"

	// claude: create old backup
	err := os.WriteFile(backupPath, []byte("old-backup-data"), 0644)
	require.NoError(t, err)

	// claude: create current log over limit
	data := make([]byte, 2*1024*1024)
	err = os.WriteFile(logPath, data, 0644)
	require.NoError(t, err)

	cfg := &Config{Logfile: logPath, MaxLogSize: 1}
	err = cfg.RotateLogfile()
	assert.NoError(t, err)

	// claude: backup should now be the rotated file, not the old one
	info, err := os.Stat(backupPath)
	assert.NoError(t, err)
	assert.Equal(t, int64(2*1024*1024), info.Size())
}

func TestConfig_Save_NewFileDefaultsTOML(t *testing.T) {
	tmpDir := t.TempDir()
	oldCwd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(oldCwd)

	cfg := &Config{}
	err := cfg.Save(map[string]interface{}{
		"registrations": []Registration{{ID: "w1", Token: "t1", URL: "http://foo:8080"}},
	})
	require.NoError(t, err)

	// claude: verify TOML is the default and contains registration with URL
	f := filepath.Join(tmpDir, "cla-worker.toml")
	cfg2, err := Load(f)
	require.NoError(t, err)
	require.Len(t, cfg2.Registrations, 1)
	assert.Equal(t, "w1", cfg2.Registrations[0].ID)
	assert.Equal(t, "http://foo:8080", cfg2.Registrations[0].URL)
}
