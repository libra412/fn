package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"plugin"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/gorilla/mux"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

// 定义函数处理类型
type Handler = func(context.Context, *log.Logger, map[string][]string, []byte) ([]byte, error)

var logger = log.New(os.Stdout, "", log.LstdFlags|log.Llongfile)

var (
	functions = make(map[string]Handler) // 存储注册的函数
	mu        sync.RWMutex               // 保护 functions 的并发安全
)

// 注册函数（线程安全）
func registerFunction(name string, h Handler) {
	mu.Lock()
	defer mu.Unlock()
	functions[name] = h
}

// 移除函数（线程安全）
func unregisterFunction(name string) {
	mu.Lock()
	defer mu.Unlock()
	delete(functions, name)
}

// 处理HTTP请求
func handleInvoke(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	fnName := vars["functionName"]
	handler, exists := functions[fnName]
	if !exists {
		http.Error(w, "Function not found", http.StatusNotFound)
		return
	}

	input, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Error reading request body", http.StatusBadRequest)
		return
	}

	// 设置超时上下文
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	begin := time.Now()
	output, err := handler(ctx, logger, r.Header, input)
	defer func() {
		logger.Printf("Function %s executed in %s, request=%s, response=%s, err=%v", fnName, time.Since(begin), string(input), string(output), err)
	}()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	w.Write(output)
}

var (
	port    int
	plugins string
)

func init() {
	flag.IntVar(&port, "port", 8080, "Port to listen on")
	flag.StringVar(&plugins, "plugins", "plugins", "Directory to watch for plugins")
	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "Usage: %s [options]\n", flag.CommandLine.Name())
		flag.PrintDefaults()
	}
}

func main() {
	flag.Parse()

	// 确保插件目录存在
	if err := os.MkdirAll(plugins, 0755); err != nil {
		logger.Fatal(err)
	}
	// 初始化加载已有插件
	initPlugins(plugins)
	// 启动插件监视器（后台运行）
	go watchPlugins(plugins)

	// 启动服务器
	r := mux.NewRouter()
	r.HandleFunc("/invoke/{functionName}", handleInvoke).Methods("POST")
	logger.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", port), r))
}

func loadPlugin(path string, functionName string) error {
	p, err := plugin.Open(path)
	if err != nil {
		return err
	}
	symbolName := "Handler" + cases.Title(language.Und).String(functionName)
	sym, err := p.Lookup(symbolName)
	if err != nil {
		return err
	}
	handler, ok := sym.(Handler)
	if !ok {
		return errors.New("invalid handler type")
	}
	registerFunction(functionName, handler)
	logger.Printf("Loaded plugin: %s -> %s", path, functionName)
	return nil
}

// 初始化时加载已有插件
func initPlugins(pluginDir string) {
	files, err := os.ReadDir(pluginDir)
	if err != nil {
		logger.Fatalf("Failed to read plugin directory: %v", err)
	}

	for _, f := range files {
		if strings.HasSuffix(f.Name(), ".so") {
			// 假设插件文件名为函数名（例如 echo.so -> 函数名 "echo"）
			functionName := strings.TrimSuffix(f.Name(), ".so")
			path := filepath.Join(pluginDir, f.Name())
			if err := loadPlugin(path, functionName); err != nil {
				logger.Printf("Failed to load plugin %s: %v", path, err)
			}
		}
	}
}

// 动态监控插件目录
func watchPlugins(pluginDir string) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		logger.Fatalf("Failed to create watcher: %v", err)
	}
	defer watcher.Close()

	// 添加监控目录
	if err := watcher.Add(pluginDir); err != nil {
		logger.Fatalf("Failed to watch directory %s: %v", pluginDir, err)
	}

	logger.Printf("Watching plugin directory: %s", pluginDir)

	// 处理事件（延迟合并重复事件）
	var (
		debounceDuration = 2 * time.Second
		debounceTimer    *time.Timer
	)

	for {
		select {
		case event, ok := <-watcher.Events:
			if !ok {
				return
			}
			// 只处理创建和删除事件
			if event.Op&(fsnotify.Create|fsnotify.Remove) == 0 {
				continue
			}

			// 防抖处理：避免短时间内的重复触发
			if debounceTimer != nil {
				debounceTimer.Stop()
			}
			debounceTimer = time.AfterFunc(debounceDuration, func() {
				handlePluginEvent(event)
			})

		case err, ok := <-watcher.Errors:
			if !ok {
				return
			}
			logger.Printf("Watcher error: %v", err)
		}
	}
}

// 处理插件文件事件
func handlePluginEvent(event fsnotify.Event) {
	fileName := filepath.Base(event.Name)
	if !strings.HasSuffix(fileName, ".so") {
		return
	}

	functionName := strings.TrimSuffix(fileName, ".so")

	switch event.Op {
	case fsnotify.Create:
		// 新插件加载
		if err := loadPlugin(event.Name, functionName); err != nil {
			logger.Printf("Failed to load new plugin %s: %v", event.Name, err)
		}

	case fsnotify.Remove:
		// 移除插件（Go 无法真正卸载插件，但可以删除注册的函数）
		unregisterFunction(functionName)
		logger.Printf("Unregistered function: %s", functionName)
	}
}
