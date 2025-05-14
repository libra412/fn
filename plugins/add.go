package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
)

// input: {"a": 1, "b": 2}
func HandlerAdd(ctx context.Context, logger *log.Logger, header map[string][]string, input []byte) ([]byte, error) {
	var data struct {
		A int `json:"a"`
		B int `json:"b"`
	}
	err := json.Unmarshal(input, &data)
	logger.Println("HandlerAdd", err)
	return []byte(fmt.Sprintf(`{"result": %d}`, data.A+data.B)), err
}
