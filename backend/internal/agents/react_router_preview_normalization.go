package agents

import (
	"path/filepath"
	"regexp"
	"strings"
)

const previewProxyBrowserRouterBasename = `{window.location.pathname.match(/^(.+?\/preview\/proxy\/[^/?#]+)/)?.[1] || "/"}`

var (
	reactRouterDomNamedImportPattern = regexp.MustCompile(`(?s)import\s*\{([^}]*)\}\s*from\s*["']react-router-dom["']`)
	jsIdentifierPattern              = regexp.MustCompile(`^[A-Za-z_$][A-Za-z0-9_$]*$`)
)

func previewProxySafeBrowserRouterOpenTag() string {
	return `<BrowserRouter basename=` + previewProxyBrowserRouterBasename + `>`
}

func normalizeGeneratedReactRouterPreviewBasename(path, content string) string {
	if strings.TrimSpace(content) == "" || !strings.Contains(content, "BrowserRouter") {
		return content
	}

	normalizedPath := strings.ToLower(filepath.ToSlash(strings.TrimSpace(path)))
	switch strings.ToLower(filepath.Ext(normalizedPath)) {
	case ".tsx", ".jsx", ".ts", ".js":
	default:
		return content
	}

	for _, localName := range browserRouterLocalNames(content) {
		content = addPreviewBasenameToRouterOpenTags(content, localName)
	}
	return content
}

func browserRouterLocalNames(content string) []string {
	names := map[string]struct{}{}
	if strings.Contains(content, "<BrowserRouter") {
		names["BrowserRouter"] = struct{}{}
	}

	for _, match := range reactRouterDomNamedImportPattern.FindAllStringSubmatch(content, -1) {
		if len(match) < 2 {
			continue
		}
		for _, rawSpec := range strings.Split(match[1], ",") {
			fields := strings.Fields(strings.TrimSpace(rawSpec))
			if len(fields) == 0 {
				continue
			}
			if fields[0] == "type" {
				fields = fields[1:]
			}
			if len(fields) == 0 || fields[0] != "BrowserRouter" {
				continue
			}

			localName := "BrowserRouter"
			if len(fields) >= 3 && fields[1] == "as" {
				localName = fields[2]
			}
			localName = strings.Trim(localName, " \t\r\n;")
			if jsIdentifierPattern.MatchString(localName) {
				names[localName] = struct{}{}
			}
		}
	}

	out := make([]string, 0, len(names))
	for name := range names {
		out = append(out, name)
	}
	return out
}

func addPreviewBasenameToRouterOpenTags(content, localName string) string {
	if localName == "" || !jsIdentifierPattern.MatchString(localName) {
		return content
	}
	openTagPattern := regexp.MustCompile(`<` + regexp.QuoteMeta(localName) + `(\s+[^>]*)?>`)
	return openTagPattern.ReplaceAllStringFunc(content, func(tag string) string {
		if regexp.MustCompile(`\bbasename\s*=`).MatchString(tag) {
			return tag
		}

		attrs := strings.TrimSuffix(strings.TrimPrefix(tag, "<"+localName), ">")
		trimmedAttrs := strings.TrimSpace(attrs)
		selfClosing := strings.HasSuffix(trimmedAttrs, "/")
		if selfClosing {
			trimmedAttrs = strings.TrimSpace(strings.TrimSuffix(trimmedAttrs, "/"))
		}

		attrText := ""
		if trimmedAttrs != "" {
			attrText = " " + trimmedAttrs
		}
		if selfClosing {
			return "<" + localName + attrText + " basename=" + previewProxyBrowserRouterBasename + " />"
		}
		return "<" + localName + attrText + " basename=" + previewProxyBrowserRouterBasename + ">"
	})
}
