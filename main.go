package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
)

// 定义数据类型
type InvocationEvent struct {
	// 元数据
	Headers   map[string]string `json:"headers"`   // 所有请求头
	Method    string            `json:"method"`    // GET/POST/PUT等
	Path      string            `json:"path"`      // 请求路径
	Query     map[string]string `json:"query"`     // 查询参数
	RequestID string            `json:"requestId"` // 请求ID

	// 内容
	Body     []byte `json:"body"`     // 原始请求体
	IsBase64 bool   `json:"isBase64"` // 二进制标识
}

var logger = log.New(os.Stdout, "", log.LstdFlags|log.Llongfile)

var (
	functions = make(map[string]string) // 存储注册的函数
	mu        sync.RWMutex              // 保护 functions 的并发安全
)

// 注册函数（线程安全）
func registerFunction(name string, h string) {
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
func invokeFunc(ctx context.Context, handleName string, input []byte) ([]byte, error) {

	cmd := exec.CommandContext(ctx, fmt.Sprintf("./%s", handleName))

	// 设置输入输出
	var stdout, stderr bytes.Buffer
	cmd.Stdin = bytes.NewReader(input)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		return nil, fmt.Errorf("error running %s: %v, stderr: %s", handleName, err, stderr.String())
	}
	return stdout.Bytes(), nil
}

func handleInvoke(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	fnName := vars["functionName"]
	handleName, exists := functions[fnName]
	if !exists {
		http.Error(w, "Function not found", http.StatusNotFound)
		logger.Printf("Functions have %+v", functions)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Error reading request body", http.StatusBadRequest)
		return
	}
	// 构建事件对象
	event := InvocationEvent{
		Headers:   headerToMap(r.Header),
		Method:    r.Method,
		Path:      r.URL.Path,
		Query:     queryToMap(r.URL.Query()),
		Body:      body,
		IsBase64:  isBinaryData(r.Header.Get("Content-Type")),
		RequestID: generateRequestID(),
	}

	// 序列化事件
	eventBytes, _ := json.Marshal(event)

	// 设置超时上下文
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	begin := time.Now()
	output, err := invokeFunc(ctx, handleName, eventBytes)
	defer func() {
		logger.Printf("Function %s executed in %s, request=%s, response=%s, err=%v", fnName, time.Since(begin), string(eventBytes), string(output), err)
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

func generateRequestID() string {
	// 生成一个简单的请求ID，可以使用UUID或其他方法
	return fmt.Sprintf("%d", uuid.New().ID())
}

// 主程序检测二进制内容
func isBinaryData(contentType string) bool {
	return !strings.Contains(contentType, "text/") &&
		!strings.Contains(contentType, "application/json")
}

// 请求头转map (处理多值)
func headerToMap(h http.Header) map[string]string {
	m := make(map[string]string)
	for k, v := range h {
		if len(v) > 0 {
			m[k] = v[0]
		}
	}
	return m
}

// 查询参数转map (处理多值)
func queryToMap(params url.Values) map[string]string {
	m := make(map[string]string)
	for k, v := range params {
		if len(v) > 0 {
			m[k] = v[0]
		}
	}
	return m
}

var (
	port    int
	funcDir string
)

func init() {
	flag.IntVar(&port, "port", 8080, "Port to listen on")
	flag.StringVar(&funcDir, "funcDir", "functions", "Directory to watch for functions")
	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "Usage: %s [options]\n", flag.CommandLine.Name())
		flag.PrintDefaults()
	}
}

func main() {
	flag.Parse()

	// 确保插件目录存在
	if err := os.MkdirAll(funcDir, 0755); err != nil {
		logger.Fatal(err)
	}
	// 初始化加载已有插件
	initFunctions(funcDir)
	// 启动插件监视器（后台运行）
	go watchPlugins(funcDir)

	// 启动服务器
	r := mux.NewRouter()
	r.HandleFunc("/invoke/{functionName}", handleInvoke).Methods("POST")
	logger.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", port), r))
}

// 初始化时加载已有插件
func initFunctions(pluginDir string) {
	files, err := os.ReadDir(pluginDir)
	if err != nil {
		logger.Fatalf("Failed to read plugin directory: %v", err)
	}

	for _, f := range files {
		functionName := f.Name()
		path := filepath.Join(pluginDir, f.Name())
		registerFunction(functionName, path)
		logger.Printf("registered Function : %s", functionName)
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

	logger.Printf("Watching functions directory: %s", pluginDir)

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
	functionName := fileName

	switch event.Op {
	case fsnotify.Create:
		// 新加载函数
		registerFunction(functionName, event.Name)
		logger.Printf("registered Function : %s", functionName)
	case fsnotify.Remove:
		// 移除函数
		unregisterFunction(functionName)
		logger.Printf("Unregistered function: %s", functionName)
	}
}
