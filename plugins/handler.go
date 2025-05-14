package plugins

import (
	"context"
	"encoding/json"
	"errors"
	"log"

	"github.com/libra412/fn/plugins/transport"
)

// Handler is the main entry point for handling requests.
// It unmarshals the request body, retrieves the appropriate endpoint from the router,
// and calls the endpoint with the provided context, logger, header, and request data.
// It returns the response as a byte slice or an error if something goes wrong.
// The request body should be a JSON object with "method" and "data" fields.
// The "method" field specifies the endpoint to call, and the "data" field contains the request data.
// The response is expected to be a JSON object as well.
// for example:
//
//	{
//	  "method": "user.get",
//	  "data": {
//	    "id": 1
//	  }
//	}
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
