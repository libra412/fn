package plugins

import (
	"context"
	"encoding/json"
	"errors"
	"log"

	"github.com/libra412/fn/plugins/transport"
)

func Handler(ctx context.Context, logger *log.Logger, header map[string][]string, body []byte) ([]byte, error) {
	var request struct {
		Method string `json:"method"`
		Data   any    `json:"data"`
	}
	if err := json.Unmarshal(body, &request); err != nil {
		logger.Println("Error unmarshalling request:", err)
		return nil, err
	}
	r := transport.GetRouter()
	endpoint := r.GetEndpoint(request.Method)
	if endpoint == nil {
		err := errors.New("endpoint not found")
		logger.Println("Error:", err)
		return nil, err
	}
	return endpoint(ctx, logger, header, request.Data)
}
