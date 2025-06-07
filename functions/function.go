package functions

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/textproto"
	"os"
	"reflect"

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

func DecodeHeaderPostRequest[T any](r T) func(header http.Header, body []byte) T {
	return func(header http.Header, body []byte) T {
		t := reflect.TypeOf(r)
		v := reflect.New(t).Elem() // 创建结构体实例

		// 处理header部分
		for i := 0; i < t.NumField(); i++ {
			field := t.Field(i)
			tag := field.Tag.Get("header")
			if tag == "" {
				continue
			}

			// 获取规范化的Header键名
			canonicalKey := textproto.CanonicalMIMEHeaderKey(tag)
			values, exists := header[canonicalKey]
			if !exists || len(values) == 0 {
				continue
			}

			fieldVal := v.Field(i)
			if !fieldVal.CanSet() {
				continue
			}

			// 根据字段类型设置值
			switch fieldVal.Kind() {
			case reflect.String:
				fieldVal.SetString(values[0]) // 取第一个值
			case reflect.Slice:
				if elemKind := field.Type.Elem().Kind(); elemKind == reflect.String {
					fieldVal.Set(reflect.ValueOf(values))
				} else {
					panic(fmt.Errorf("unsupported slice type %s for header field %s", elemKind, field.Name))
				}
			case reflect.Ptr:
				if field.Type.Elem().Kind() == reflect.String {
					strVal := values[0]
					fieldVal.Set(reflect.ValueOf(&strVal))
				} else {
					panic(fmt.Errorf("unsupported pointer type %s for header field %s", field.Type.Elem().Kind(), field.Name))
				}
			default:
				panic(fmt.Errorf("unsupported type %s for header field %s", fieldVal.Kind(), field.Name))
			}
		}

		// 处理body部分（保持不变）
		if len(body) != 0 {
			if err := json.NewDecoder(bytes.NewReader(body)).Decode(v.Addr().Interface()); err != nil {
				panic(err)
			}
		}

		return v.Interface().(T)
	}
}
