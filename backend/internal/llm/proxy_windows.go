//go:build windows

package llm

import (
	"log"
	"net/http"
	"net/url"
	"strings"

	"golang.org/x/sys/windows/registry"
)

// winINETSettingsKey is where Internet Options stores the proxy configuration.
const winINETSettingsKey = `Software\Microsoft\Windows\CurrentVersion\Internet Settings`

// osProxy reads the proxy configured in Windows Internet Options and returns nil
// when it is disabled.
//
// Only HKCU (the current user) is read: a desktop app should follow the signed-in
// user's settings, and this needs no extra privileges. ProxyServer may hold a value
// while the proxy is turned off, so ProxyEnable is checked first.
func osProxy() func(*http.Request) (*url.URL, error) {
	key, err := registry.OpenKey(registry.CURRENT_USER, winINETSettingsKey, registry.QUERY_VALUE)
	if err != nil {
		log.Printf("[LLM] failed to read the system proxy (cannot open Internet Settings): %v", err)
		return nil
	}
	defer key.Close()

	enabled, _, err := key.GetIntegerValue("ProxyEnable")
	if err != nil || enabled == 0 {
		return nil
	}

	server, _, err := key.GetStringValue("ProxyServer")
	if err != nil || strings.TrimSpace(server) == "" {
		return nil
	}
	// A missing ProxyOverride is normal and means there is no bypass list.
	override, _, _ := key.GetStringValue("ProxyOverride")

	cfg := winINETProxyConfig(server, override)
	if cfg == nil {
		return nil
	}

	proxyForURL := cfg.ProxyFunc()
	return func(req *http.Request) (*url.URL, error) {
		if req == nil || req.URL == nil {
			return nil, nil
		}
		return proxyForURL(req.URL)
	}
}
