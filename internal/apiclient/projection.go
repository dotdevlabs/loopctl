package apiclient

import (
	"context"
	"encoding/json"
	"net/http"
)

const responseObserverKey contextKey = 3

type ResponseObserver func(response *http.Response, path string, projection any)

func WithResponseObserver(ctx context.Context, observer ResponseObserver) context.Context {
	return context.WithValue(ctx, responseObserverKey, observer)
}

func ObserveResponse(resp *http.Response, path string, projection any) {
	if resp == nil || resp.Request == nil {
		return
	}
	if observer, ok := resp.Request.Context().Value(responseObserverKey).(ResponseObserver); ok {
		observer(resp, path, projection)
	}
}

func DecodeResponse(resp *http.Response, path string, data []byte, target any) error {
	ObserveResponse(resp, path, target)
	return json.Unmarshal(data, target)
}
