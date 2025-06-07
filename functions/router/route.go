package router

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/libra412/fn/v2/functions/logger"
	"github.com/libra412/fn/v2/functions/request"
)

type Router func(logger *log.Logger, query map[string]string, headers http.Header, body []byte) (any, error)

func Process(event request.InvocationEvent, router Router) ([]byte, error) {
	// 处理业务逻辑
	result, err := router(logger.GetLogger(), event.Query, event.Headers, event.Body)
	if err != nil {
		// 处理错误
		errorResponse := map[string]any{"error_no": 1, "msg": err.Error()}
		return json.Marshal(errorResponse)
	}

	// 处理成功
	response := map[string]any{"error_no": 0, "msg": "success", "data": result}
	return json.Marshal(response)
}
