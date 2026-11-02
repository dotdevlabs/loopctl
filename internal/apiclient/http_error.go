package apiclient

// HTTPError preserves a failed GET's status while retaining its original error.
type HTTPError struct {
	StatusCode int
	Err        error
}

func (e *HTTPError) Error() string { return e.Err.Error() }
func (e *HTTPError) Unwrap() error { return e.Err }
