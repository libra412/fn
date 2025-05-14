package endpoint

import (
	"context"
	"encoding/json"
	"log"
	"reflect"
)

type Endpoint func(ctx context.Context, logger *log.Logger, header map[string][]string, request any) (response []byte, err error)

type Response struct {
	ErrorNo  int    `json:"error_no"`
	ErrorMsg string `json:"error_msg"`
	Data     any    `json:"data"`
}

func (r Response) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func ResponseError(err error) ([]byte, error) {
	return Response{
		ErrorNo:  1,
		ErrorMsg: err.Error(),
		Data:     nil,
	}.Marshal()
}

func ResponseSuccess(data any) ([]byte, error) {
	return Response{
		ErrorNo:  0,
		ErrorMsg: "",
		Data:     data,
	}.Marshal()
}

func DecodePostRequest[T any](st T, body any) T {
	t := reflect.TypeOf(st)
	v := reflect.New(t)
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			panic(err)
		}
		if err := json.Unmarshal(b, v.Interface()); err != nil {
			panic(err)
		}
	}
	d := v.Elem().Interface()
	return d.(T)
}
