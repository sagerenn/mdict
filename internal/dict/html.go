package dict

import (
	"net/url"
	"path"
	"regexp"
	"strings"
)

var attrRe = regexp.MustCompile(`(?i)(\b(?:src|href)\s*=\s*)(\"|')([^\"']+)(\"|')`)

// EntryURL builds a local URL for HTML entry links.
func EntryURL(dictID, word string) string {
	return "/entry?dict=" + url.QueryEscape(dictID) + "&q=" + url.QueryEscape(word)
}

// ResourceURL builds a local URL for HTML resource links.
func ResourceURL(dictID, name string) string {
	return "/resource?dict=" + url.QueryEscape(dictID) + "&name=" + url.QueryEscape(name)
}

// RewriteResourceLinks rewrites src/href URLs to local endpoints when possible.
func RewriteResourceLinks(html, dictID string) string {
	if html == "" {
		return html
	}
	var out strings.Builder
	out.Grow(len(html))
	last := 0
	for _, loc := range attrRe.FindAllStringSubmatchIndex(html, -1) {
		if len(loc) < 10 {
			continue
		}
		out.WriteString(html[last:loc[0]])

		attrPrefix := html[loc[2]:loc[3]]
		quoteOpen := html[loc[4]:loc[5]]
		rawURL := html[loc[6]:loc[7]]
		quoteClose := html[loc[8]:loc[9]]

		newURL := rewriteURL(rawURL, dictID)
		out.WriteString(attrPrefix)
		out.WriteString(quoteOpen)
		out.WriteString(newURL)
		out.WriteString(quoteClose)

		last = loc[1]
	}
	out.WriteString(html[last:])
	return out.String()
}

func rewriteURL(rawURL, dictID string) string {
	u := strings.TrimSpace(rawURL)
	if u == "" {
		return rawURL
	}

	// Preserve anchors and already rewritten paths.
	if strings.HasPrefix(u, "#") || strings.HasPrefix(u, "/resource") || strings.HasPrefix(u, "/entry") {
		return rawURL
	}

	// Handle internal schemes first.
	switch {
	case strings.HasPrefix(u, "entry://"):
		return entryURLWithAnchor(dictID, u[len("entry://"):])
	case strings.HasPrefix(u, "bword://"):
		return entryURLWithAnchor(dictID, u[len("bword://"):])
	case strings.HasPrefix(u, "bword:"):
		return entryURLWithAnchor(dictID, u[len("bword:"):])
	case strings.HasPrefix(u, "sound://"):
		return ResourceURL(dictID, decodePath(u[len("sound://"):]))
	}

	// Leave fully qualified URLs and known schemes unchanged.
	lower := strings.ToLower(u)
	if strings.Contains(lower, "://") ||
		strings.HasPrefix(lower, "data:") ||
		strings.HasPrefix(lower, "mailto:") ||
		strings.HasPrefix(lower, "bres://") ||
		strings.HasPrefix(lower, "gdau://") ||
		strings.HasPrefix(lower, "gdlookup://") ||
		strings.HasPrefix(lower, "qrc://") ||
		strings.HasPrefix(lower, "qrcx://") {
		return rawURL
	}

	// Otherwise treat as relative resource.
	return ResourceURL(dictID, decodePath(u))
}

func decodePath(p string) string {
	p = strings.SplitN(p, "?", 2)[0]
	p = strings.SplitN(p, "#", 2)[0]
	if decoded, err := url.PathUnescape(p); err == nil {
		return decoded
	}
	return p
}

func entryURLWithAnchor(dictID, raw string) string {
	parts := strings.SplitN(raw, "#", 2)
	word := decodePath(parts[0])
	urlStr := EntryURL(dictID, word)
	if len(parts) == 2 && parts[1] != "" {
		urlStr += "#" + parts[1]
	}
	return urlStr
}

// CleanResourceName normalizes a resource path and prevents directory traversal.
func CleanResourceName(name string) string {
	if name == "" {
		return ""
	}
	if decoded, err := url.PathUnescape(name); err == nil {
		name = decoded
	}
	clean := path.Clean("/" + name)
	if strings.HasPrefix(clean, "/..") {
		return ""
	}
	return strings.TrimPrefix(clean, "/")
}
