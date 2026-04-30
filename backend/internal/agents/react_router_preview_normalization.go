package agents

import (
	"path/filepath"
	"regexp"
	"strings"
)

const previewProxyBrowserRouterBasename = `{window.location.pathname.match(/^(.+?\/preview\/proxy\/[^/?#]+)/)?.[1] || "/"}`

var browserRouterOpenTagPattern = regexp.MustCompile(`<BrowserRouter(\s+[^>]*)?>`)

func previewProxySafeBrowserRouterOpenTag() string {
	return `<BrowserRouter basename=` + previewProxyBrowserRouterBasename + `>`
}

func normalizeGeneratedReactRouterPreviewBasename(path, content string) string {
	if strings.TrimSpace(content) == "" || !strings.Contains(content, "BrowserRouter") || !strings.Contains(content, "<BrowserRouter") {
		return content
	}

	normalizedPath := strings.ToLower(filepath.ToSlash(strings.TrimSpace(path)))
	switch strings.ToLower(filepath.Ext(normalizedPath)) {
	case ".tsx", ".jsx", ".ts", ".js":
	default:
		return content
	}

	return browserRouterOpenTagPattern.ReplaceAllStringFunc(content, func(tag string) string {
		if strings.Contains(tag, "basename=") || strings.Contains(tag, "basename =") {
			return tag
		}
		attrs := strings.TrimSuffix(strings.TrimPrefix(tag, "<BrowserRouter"), ">")
		return "<BrowserRouter" + attrs + " basename=" + previewProxyBrowserRouterBasename + ">"
	})
}
