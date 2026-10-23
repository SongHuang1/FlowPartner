//go:build !windows

package llm

import (
	"net/http"
	"net/url"
)

// osProxy has no system-level implementation on non-Windows platforms. These platforms can
// still configure a proxy through the HTTP_PROXY / HTTPS_PROXY / NO_PROXY
// environment variables, which systemProxy handles.
func osProxy() func(*http.Request) (*url.URL, error) {
	return nil
}
