package tasks

import (
	"context"
	"errors"
	"io"
	"net"
	"time"

	"github.com/dotdevlabs/loopctl/internal/apiclient"
)

func retryableWatchError(err error) bool {
	if errors.Is(err, context.Canceled) {
		return false
	}
	var status *apiclient.HTTPError
	if errors.As(err, &status) {
		return status.StatusCode == 408 || status.StatusCode == 429 || (status.StatusCode >= 500 && status.StatusCode <= 599)
	}
	if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) || errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	var network net.Error
	if errors.As(err, &network) && network.Timeout() {
		return true
	}
	var operation *net.OpError
	return errors.As(err, &operation)
}

func nextWatchDelay(previous, interval time.Duration) time.Duration {
	if previous == 0 {
		if interval < time.Second {
			return interval
		}
		return time.Second
	}
	if previous >= interval-previous {
		return interval
	}
	return previous * 2
}
