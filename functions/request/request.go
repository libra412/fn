package request

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
