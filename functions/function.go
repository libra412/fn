package functions

import (
	"encoding/json"
	"io"
	"os"

	"github.com/libra412/fn/functions/request"
	"github.com/libra412/fn/functions/router"
)

func ExecuteFunction(route router.Router) {
	// 读取标准输入
	input, _ := io.ReadAll(os.Stdin)

	// 反序列化事件
	var event request.InvocationEvent
	json.Unmarshal(input, &event)

	// 处理业务逻辑
	result, err := router.Process(event, route)
	if err != nil {
		// 处理错误
		errorResponse := map[string]any{"error_no": 1, "msg": err.Error()}
		result, _ = json.Marshal(errorResponse)
	}

	// 返回响应
	os.Stdout.Write(result)
}
