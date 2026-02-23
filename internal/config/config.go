package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// ProviderConfig 定义单个 AI 提供商配置
type ProviderConfig struct {
	Name           string   `json:"name"`
	APIKey         string   `json:"api_key"`
	BaseURL        string   `json:"base_url"`
	DefaultModel   string   `json:"default_model"`
	FallbackModels []string `json:"fallback_models"`
}

// AIConfig AI 模型提供商配置
type AIConfig struct {
	Providers       map[string]ProviderConfig `json:"providers"` // 键: "openai", "anthropic", "deepseek", "openrouter" 等
	DefaultProvider string                    `json:"default_provider"`
}

// Config 完整配置结构（完全本地模式）
type Config struct {
	RootDir              string   `json:"root_dir"` // 工作区根目录（空则使用当前目录）
	AI                   AIConfig `json:"ai"`
	LogLevel             string   `json:"log_level"`
	MaxSearchResults     int      `json:"max_search_results"`
	MaxFileBytes         int64    `json:"max_file_bytes"`
	BuildTimeout         int64    `json:"build_timeout_seconds"`  // 构建超时时间（秒）
	AllowedBuildCommands []string `json:"allowed_build_commands"` // 允许的构建命令列表（白名单）
	AllowedPaths         []string `json:"allowed_paths"`          // 允许操作的目录白名单（空表示不限制）
	BlockedExtensions    []string `json:"blocked_extensions"`     // 拦截的文件扩展名黑名单
	LowResourceMode      bool     `json:"low_resource_mode"`      // 低功耗模式（针对树莓派）
	ConfigFile           string   `json:"-"`                      // 记住配置文件来源
}

// 默认值
const (
	DefaultLogLevel         = "info"
	DefaultMaxSearchResults = 50
	DefaultMaxFileBytes     = 1024 * 1024 // 1 MB
)

// 预置的提供商配置模板
var builtinProviders = map[string]ProviderConfig{
	"gemini": {
		Name:           "Google Gemini",
		BaseURL:        "https://generativelanguage.googleapis.com/v1beta",
		DefaultModel:   "gemini-2.0-pro-exp-02-05",
		FallbackModels: []string{"gemini-1.5-pro", "gemini-1.5-flash"},
	},
	"anthropic": {
		Name:           "Anthropic",
		BaseURL:        "https://api.anthropic.com",
		DefaultModel:   "claude-3-5-sonnet-latest",
		FallbackModels: []string{"claude-3-opus-latest", "claude-3-haiku-20240307"},
	},
	"deepseek": {
		Name:           "DeepSeek",
		BaseURL:        "https://api.deepseek.com",
		DefaultModel:   "deepseek-chat",
		FallbackModels: []string{"deepseek-reasoner"},
	},
	"openrouter": {
		Name:           "OpenRouter",
		BaseURL:        "https://openrouter.ai/api/v1",
		DefaultModel:   "openrouter/auto",
		FallbackModels: []string{"openrouter/llama-3.2-11b-vision-instruct", "openrouter/mistral-7b-instruct"},
	},
}

// LoadConfig 加载配置（环境变量 + 配置文件）
func LoadConfig() (*Config, error) {
	// 1. 从默认位置加载配置文件（如果存在）
	cfg := &Config{}
	cfg.setDefaults()

	configPaths := []string{
		"./config.json",
		"./config.yaml",
		os.ExpandEnv("$HOME/.config/agentcode-mcp/config.json"),
		os.ExpandEnv("$HOME/.config/agentcode-mcp/config.yaml"),
		"/etc/agentcode-mcp/config.json",
		"/etc/agentcode-mcp/config.yaml",
	}

	loaded := false
	for _, path := range configPaths {
		if _, err := os.Stat(path); err == nil {
			if err := loadFromFile(path, cfg); err != nil {
				return nil, fmt.Errorf("failed to load config from %s: %w", path, err)
			}
			cfg.ConfigFile = path
			loaded = true
			break
		}
	}

	// 如果未找到配置文件，自动生成占位配置文件到用户主目录
	if !loaded {
		if err := createPlaceholderConfig(cfg); err != nil {
			return nil, fmt.Errorf("failed to create placeholder config: %w", err)
		}
	}

	// 2. 环境变量覆盖
	applyEnvOverrides(cfg)

	// 3. 确保必填字段
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

// setDefaults 设置默认值
func (c *Config) setDefaults() {
	c.RootDir = "" // 默认使用当前工作目录
	c.LogLevel = DefaultLogLevel
	c.MaxSearchResults = DefaultMaxSearchResults
	c.MaxFileBytes = DefaultMaxFileBytes
	c.BuildTimeout = 60 // 默认 60 秒
	c.AllowedBuildCommands = []string{"go build", "go test", "go vet", "go mod tidy", "go run"}

	// 安全相关默认值
	c.AllowedPaths = []string{} // 空表示不限制（生产环境应配置）
	c.BlockedExtensions = []string{".env", ".key", ".pem", ".crt", ".cer", ".p12", ".pfx", ".jks", ".keystore"}
	c.LowResourceMode = false

	// 初始化 AI 配置，包含预置提供商
	c.AI = AIConfig{
		Providers:       make(map[string]ProviderConfig),
		DefaultProvider: "gemini",
	}
	// 复制内置模板（避免修改原模板）
	for k, v := range builtinProviders {
		c.AI.Providers[k] = v
	}
}

// loadFromFile 从文件加载配置
func loadFromFile(path string, cfg *Config) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	if strings.HasSuffix(path, ".yaml") || strings.HasSuffix(path, ".yml") {
		// 需要 yaml 支持，暂时用 JSON 替代或使用 go-yaml（此处简化）
		return fmt.Errorf("yaml support not yet implemented, use json")
	}

	// JSON
	var partial struct {
		AI                   AIConfig `json:"ai"`
		LogLevel             string   `json:"log_level"`
		MaxSearchResults     int      `json:"max_search_results"`
		MaxFileBytes         int64    `json:"max_file_bytes"`
		BuildTimeout         int64    `json:"build_timeout_seconds"`
		AllowedBuildCommands []string `json:"allowed_build_commands"`
		AllowedPaths         []string `json:"allowed_paths"`
		BlockedExtensions    []string `json:"blocked_extensions"`
		LowResourceMode      bool     `json:"low_resource_mode"`
	}
	if err := json.Unmarshal(data, &partial); err != nil {
		return err
	}

	// 合并配置（保留未设置的默认值）
	if partial.LogLevel != "" {
		cfg.LogLevel = partial.LogLevel
	}
	if partial.MaxSearchResults > 0 {
		cfg.MaxSearchResults = partial.MaxSearchResults
	}
	if partial.MaxFileBytes > 0 {
		cfg.MaxFileBytes = partial.MaxFileBytes
	}
	if partial.AI.DefaultProvider != "" {
		cfg.AI.DefaultProvider = partial.AI.DefaultProvider
	}
	// 合并提供商配置（部分更新）
	for k, v := range partial.AI.Providers {
		existing, exists := cfg.AI.Providers[k]
		if !exists {
			cfg.AI.Providers[k] = v
			continue
		}
		// 更新非空字段
		if v.Name != "" {
			existing.Name = v.Name
		}
		if v.BaseURL != "" {
			existing.BaseURL = v.BaseURL
		}
		if v.DefaultModel != "" {
			existing.DefaultModel = v.DefaultModel
		}
		if len(v.FallbackModels) > 0 {
			existing.FallbackModels = v.FallbackModels
		}
		if v.APIKey != "" {
			existing.APIKey = v.APIKey
		}
		cfg.AI.Providers[k] = existing
	}

	// 合并构建配置
	if partial.BuildTimeout > 0 {
		cfg.BuildTimeout = partial.BuildTimeout
	}
	if len(partial.AllowedBuildCommands) > 0 {
		cfg.AllowedBuildCommands = partial.AllowedBuildCommands
	}
	// 安全配置
	if len(partial.AllowedPaths) > 0 {
		cfg.AllowedPaths = partial.AllowedPaths
	}
	if len(partial.BlockedExtensions) > 0 {
		cfg.BlockedExtensions = partial.BlockedExtensions
	}
	if partial.LowResourceMode {
		cfg.LowResourceMode = partial.LowResourceMode
	}

	return nil
}

// applyEnvOverrides 应用环境变量覆盖配置（本地模式）
func applyEnvOverrides(cfg *Config) {
	// 通用配置（使用新的环境变量名或兼容旧的？任务要求移除 OPCODE_*，这里保留通用配置但移除前缀）
	// 为了向后兼容，暂时仍接受 OPCODE_ 前缀，但会逐步淘汰
	if lvl := os.Getenv("LOG_LEVEL"); lvl != "" {
		cfg.LogLevel = lvl
	} else if lvl := os.Getenv("OPCODE_LOG_LEVEL"); lvl != "" {
		cfg.LogLevel = lvl
	}
	if v := getEnvInt("MAX_SEARCH_RESULTS", 0); v > 0 {
		cfg.MaxSearchResults = v
	} else if v := getEnvInt("OPCODE_MAX_SEARCH_RESULTS", 0); v > 0 {
		cfg.MaxSearchResults = v
	}
	if v := getEnvInt64("MAX_FILE_BYTES", 0); v > 0 {
		cfg.MaxFileBytes = v
	} else if v := getEnvInt64("OPCODE_MAX_FILE_BYTES", 0); v > 0 {
		cfg.MaxFileBytes = v
	}
	if v := getEnvInt64("BUILD_TIMEOUT_SECONDS", 0); v > 0 {
		cfg.BuildTimeout = v
	} else if v := getEnvInt64("OPCODE_BUILD_TIMEOUT_SECONDS", 0); v > 0 {
		cfg.BuildTimeout = v
	}
	// AllowedBuildCommands 不支持环境变量（通常是列表），从配置文件读取

	// AI 提供商特定环境变量（可选）
	// 格式：AI_<PROVIDER>_API_KEY, AI_<PROVIDER>_DEFAULT_MODEL
	for providerName := range cfg.AI.Providers {
		envKey := fmt.Sprintf("AI_%s_API_KEY", strings.ToUpper(providerName))
		if key := os.Getenv(envKey); key != "" {
			prov := cfg.AI.Providers[providerName]
			prov.APIKey = key
			cfg.AI.Providers[providerName] = prov
		}
		envModel := fmt.Sprintf("AI_%s_DEFAULT_MODEL", strings.ToUpper(providerName))
		if model := os.Getenv(envModel); model != "" {
			prov := cfg.AI.Providers[providerName]
			prov.DefaultModel = model
			cfg.AI.Providers[providerName] = prov
		}
	}
}

// Validate 验证配置
func (c *Config) Validate() error {
	var errs []error

	// 本地模式无需 OpenCode Token

	if c.MaxSearchResults <= 0 {
		errs = append(errs, &configError{field: "MaxSearchResults", message: "must be positive"})
	}
	if c.MaxFileBytes <= 0 {
		errs = append(errs, &configError{field: "MaxFileBytes", message: "must be positive"})
	}
	if c.BuildTimeout <= 0 {
		errs = append(errs, &configError{field: "BuildTimeout", message: "must be positive"})
	}
	if len(c.AllowedBuildCommands) == 0 {
		errs = append(errs, &configError{field: "AllowedBuildCommands", message: "cannot be empty"})
	}

	if len(errs) == 0 {
		return nil
	}
	return &validationError{errors: errs}
}

// createPlaceholderConfig 创建占位配置文件到用户主目录
func createPlaceholderConfig(cfg *Config) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("cannot determine home directory: %w", err)
	}
	configDir := filepath.Join(home, ".config", "agentcode-mcp")
	configPath := filepath.Join(configDir, "config.json")

	// 确保目录存在
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("failed to create config dir: %w", err)
	}

	// 生成占位 JSON（本地模式配置）
	placeholder := &Config{
		LogLevel:             DefaultLogLevel,
		MaxSearchResults:     DefaultMaxSearchResults,
		MaxFileBytes:         DefaultMaxFileBytes,
		BuildTimeout:         60,
		AllowedBuildCommands: []string{"go build", "go test", "go vet", "go mod tidy", "go run"},
		AllowedPaths:         []string{},
		BlockedExtensions:    []string{".env", ".key", ".pem", ".crt", ".cer", ".p12", ".pfx", ".jks", ".keystore"},
		LowResourceMode:      false,
	}

	data, err := json.MarshalIndent(placeholder, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal placeholder config: %w", err)
	}

	// 写入文件
	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write placeholder config: %w", err)
	}

	cfg.ConfigFile = configPath
	// 将默认值填充到 cfg
	cfg.LogLevel = placeholder.LogLevel
	cfg.MaxSearchResults = placeholder.MaxSearchResults
	cfg.MaxFileBytes = placeholder.MaxFileBytes

	return nil
}

// --- 辅助函数（复用原有） ---

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	valStr := os.Getenv(key)
	if valStr == "" {
		return defaultValue
	}
	val, err := strconv.Atoi(valStr)
	if err != nil || val <= 0 {
		return defaultValue
	}
	return val
}

func getEnvInt64(key string, defaultValue int64) int64 {
	valStr := os.Getenv(key)
	if valStr == "" {
		return defaultValue
	}
	val, err := strconv.ParseInt(valStr, 10, 64)
	if err != nil || val <= 0 {
		return defaultValue
	}
	return val
}

// 自定义错误类型
type configError struct {
	field   string
	message string
}

func (e *configError) Error() string {
	return e.field + " " + e.message
}

type validationError struct {
	errors []error
}

func (e *validationError) Error() string {
	if len(e.errors) == 1 {
		return e.errors[0].Error()
	}
	msgs := make([]string, len(e.errors))
	for i, err := range e.errors {
		msgs[i] = err.Error()
	}
	return "multiple configuration errors: " + join(msgs, "; ")
}

func join(ss []string, sep string) string {
	if len(ss) == 0 {
		return ""
	}
	result := ss[0]
	for _, s := range ss[1:] {
		result += sep + s
	}
	return result
}
