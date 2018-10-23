package client

import "fmt"

// HTTPResult composes the result of an HTTP request
type HTTPResult struct {
	Code int
	Body []byte
}

func (res *HTTPResult) String() string {
	return fmt.Sprintf("[%d]: %s", res.Code, string(res.Body))
}
