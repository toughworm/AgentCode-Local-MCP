package config

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"

	"github.com/fsnotify/fsnotify"
)

// ReloadCallback 配置重载时的回调函数类型
type ReloadCallback func(oldCfg, newCfg *Config) error

// Reloader 配置热重载管理器
type Reloader struct {
	mu        sync.RWMutex
	cfg       *Config
	configPath string
	watcher   *fsnotify.Watcher
	callbacks []ReloadCallback
	stopChan  chan struct{}
	running   bool
}

// NewReloader 创建配置重载器
func NewReloader(cfg *Config, configPath string) (*Reloader, error) {
	// 确保配置文件路径是绝对的
	absPath, err := filepath.Abs(configPath)
	if err != nil {
		return nil, fmt.Errorf("invalid config path: %w", err)
	}

	// 检查配置文件是否存在
	if _, err := os.Stat(absPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("config file does not exist: %s", absPath)
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("failed to create watcher: %w", err)
	}

	r := &Reloader{
		cfg:        cfg,
		configPath: absPath,
		watcher:    watcher,
		stopChan:   make(chan struct{}),
		callbacks:  make([]ReloadCallback, 0),
	}

	// 监视配置文件所在的目录（而不是文件本身，因为文件重命名时不会触发事件）
	dir := filepath.Dir(absPath)
	if err := watcher.Add(dir); err != nil {
		watcher.Close()
		return nil, fmt.Errorf("failed to watch config directory: %w", err)
	}

	return r, nil
}

// Start 启动配置监视后台 goroutine
func (r *Reloader) Start() error {
	if r.running {
		return fmt.Errorf("reloader already running")
	}
	r.running = true

	go r.watchLoop()
	return nil
}

// Stop 停止配置监视
func (r *Reloader) Stop() error {
	if !r.running {
		return fmt.Errorf("reloader not running")
	}
	close(r.stopChan)
	r.running = false
	return r.watcher.Close()
}

// watchLoop 监视文件系统事件的主循环
func (r *Reloader) watchLoop() {
	for {
		select {
		case <-r.stopChan:
			return
		case event, ok := <-r.watcher.Events:
			if !ok {
				return
			}
			// 检查是否为我们的配置文件
			if filepath.Clean(event.Name) != filepath.Clean(r.configPath) {
				continue
			}

			// 只处理写入和重命名/创建事件
			if event.Op&(fsnotify.Write|fsnotify.Create|fsnotify.Rename) != 0 {
				// 延迟一小段时间确保文件完全写入
				// 实际中可能需要更复杂的处理，这里简单 sleep
				// 为避免阻塞事件循环，在单独 goroutine 中执行重载
				go func() {
					if err := r.reload(); err != nil {
						log.Printf("[config] reload failed: %v", err)
					} else {
						log.Printf("[config] reloaded successfully from %s", r.configPath)
					}
				}()
			}

		case err, ok := <-r.watcher.Errors:
			if !ok {
				return
			}
			log.Printf("[config] watcher error: %v", err)
		}
	}
}

// reload 执行配置重载
func (r *Reloader) reload() error {
	// 1. 加载新配置
	newCfg := &Config{}
	newCfg.setDefaults()

	// 从文件加载（注意：不创建占位符，如果文件有问题则失败）
	if err := loadFromFile(r.configPath, newCfg); err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// 2. 应用环境变量覆盖（可选：是否需要？热重载通常只从文件加载）
	// 如果希望在热重载时也应用环境变量，取消下面这行
	// applyEnvOverrides(newCfg)

	// 3. 验证新配置
	if err := newCfg.Validate(); err != nil {
		return fmt.Errorf("invalid config: %w", err)
	}

	// 4. 原子性地更新配置（加锁）
	r.mu.Lock()
	oldCfg := r.cfg
	r.cfg = newCfg
	r.mu.Unlock()

	// 5. 触发回调（让各个组件更新自己的状态）
	for _, cb := range r.callbacks {
		if err := cb(oldCfg, newCfg); err != nil {
			log.Printf("[config] callback failed: %v", err)
		}
	}

	return nil
}

// GetConfig 获取当前配置（线程安全）
func (r *Reloader) GetConfig() *Config {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.cfg
}

// RegisterCallback 注册配置重载回调
func (r *Reloader) RegisterCallback(cb ReloadCallback) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.callbacks = append(r.callbacks, cb)
}

// ConfigPath 返回监视的配置文件路径
func (r *Reloader) ConfigPath() string {
	return r.configPath
}

// Running 返回重载器是否正在运行
func (r *Reloader) Running() bool {
	return r.running
}
