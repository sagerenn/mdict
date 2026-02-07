package dict

import (
	"regexp"
	"strings"
)

var (
	cssURLRe     = regexp.MustCompile(`(?i)url\(\s*(['"]?)([^'")]+)['"]?\s*\)`)
	cssCommentRe = regexp.MustCompile(`(?s)/\*.*?\*/`)
	rootSelector = regexp.MustCompile(`(?i):root\s*\{`)
)

// ScopeID sanitizes dictionary IDs for safe use in CSS selectors and HTML ids.
func ScopeID(dictID string) string {
	if dictID == "" {
		return "gd"
	}
	var b strings.Builder
	b.Grow(len(dictID))
	for _, r := range dictID {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			b.WriteRune(r)
		} else {
			b.WriteByte('_')
		}
	}
	if b.Len() == 0 {
		return "gd"
	}
	return b.String()
}

// RewriteCSSLinks rewrites url(...) references to the gdapi resource endpoint.
func RewriteCSSLinks(css, dictID string) string {
	if css == "" {
		return css
	}
	var out strings.Builder
	last := 0
	for _, loc := range cssURLRe.FindAllStringSubmatchIndex(css, -1) {
		if len(loc) < 6 {
			continue
		}
		out.WriteString(css[last:loc[0]])
		quote := css[loc[2]:loc[3]]
		url := strings.TrimSpace(css[loc[4]:loc[5]])
		if url == "" || isExternalCSSURL(url) {
			out.WriteString(css[loc[0]:loc[1]])
		} else {
			out.WriteString("url(" + quote + ResourceURL(dictID, url) + quote + ")")
		}
		last = loc[1]
	}
	out.WriteString(css[last:])
	return out.String()
}

func isExternalCSSURL(url string) bool {
	lower := strings.ToLower(url)
	if strings.Contains(lower, "://") {
		return true
	}
	if strings.HasPrefix(lower, "data:") {
		return true
	}
	if strings.Contains(url, ":/") {
		return true
	}
	return false
}

// IsolateCSS scopes CSS selectors to the dictionary article wrapper.
func IsolateCSS(css, dictID, wrapperSelector string) string {
	if css == "" || !strings.Contains(css, "{") {
		return css
	}

	css = cssCommentRe.ReplaceAllString(css, "")
	css = rootSelector.ReplaceAllString(css, "html{")

	prefix := "#gdarticlefrom-" + ScopeID(dictID)
	if wrapperSelector != "" {
		prefix += " " + wrapperSelector
	}

	var out strings.Builder
	out.Grow(len(css))
	current := 0
	for current < len(css) {
		ch := css[current]
		switch {
		case ch == '@':
			ruleNameEnd := current + 1
			for ruleNameEnd < len(css) {
				c := css[ruleNameEnd]
				if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_' || c == '-' {
					ruleNameEnd++
					continue
				}
				break
			}
			if ruleNameEnd == current+1 {
				out.WriteString(css[current:])
				return out.String()
			}
			ruleName := strings.TrimSpace(css[current:ruleNameEnd])

			if strings.EqualFold(ruleName, "@import") || strings.EqualFold(ruleName, "@charset") {
				if semi := strings.IndexByte(css[current:], ';'); semi >= 0 {
					semi += current
					out.WriteString(css[current : semi+1])
					current = semi + 1
					continue
				}
			}

			if strings.EqualFold(ruleName, "@page") {
				if closeBrace := findMatchingBrace(css, current); closeBrace != -1 {
					current = closeBrace + 1
					continue
				}
			}

			openBrace := strings.IndexByte(css[current:], '{')
			if openBrace >= 0 {
				openBrace += current
			}
			semi := strings.IndexByte(css[current:], ';')
			if semi >= 0 {
				semi += current
			}

			if openBrace != -1 && (semi == -1 || openBrace < semi) {
				out.WriteString(css[current : openBrace+1])
				closeBrace := findMatchingBrace(css, openBrace)
				if closeBrace != -1 {
					inner := css[openBrace+1 : closeBrace]
					inner = IsolateCSS(inner, dictID, wrapperSelector)
					out.WriteString(inner)
					out.WriteByte('}')
					current = closeBrace + 1
				} else {
					current = openBrace + 1
				}
				continue
			}
			if semi != -1 {
				out.WriteString(css[current : semi+1])
				current = semi + 1
				continue
			}

			out.WriteString(css[current:])
			return out.String()

		case ch == '{':
			if closeBrace := findMatchingBrace(css, current); closeBrace != -1 {
				out.WriteString(css[current : closeBrace+1])
				current = closeBrace + 1
				continue
			}
			out.WriteString(css[current:])
			return out.String()

		case isSelectorStart(ch):
			if (isAlpha(ch) || ch == '*') && hasNamespacePrefix(css, current, &out) {
				for current < len(css) && css[current] != '|' {
					current++
				}
				if current < len(css) {
					current++
				}
				continue
			}
			if ch == '[' {
				if end := findMatchingSquare(css, current); end != -1 {
					out.WriteString(prefix + " ")
					out.WriteString(css[current : end+1])
					current = end + 1
					continue
				}
			}
			selectorEnd := indexSelectorSeparator(css, current+1)
			if selectorEnd < 0 {
				selectorEnd = indexSelectorEnd(css, current)
			}
			selectorPart := css[current:]
			if selectorEnd >= 0 {
				selectorPart = css[current:selectorEnd]
			}
			if selectorEnd < 0 {
				out.WriteString(prefix + " " + selectorPart)
				return out.String()
			}
			trimmed := strings.TrimSpace(selectorPart)
			if strings.EqualFold(trimmed, "body") || strings.EqualFold(trimmed, "html") {
				out.WriteString(selectorPart + " " + prefix + " ")
				current += len(trimmed)
			} else {
				out.WriteString(prefix + " ")
			}
			ruleStart := indexSelectorEnd(css, current)
			remaining := css[current:]
			if ruleStart >= 0 {
				remaining = css[current:ruleStart]
			}
			out.WriteString(remaining)
			if ruleStart < 0 {
				return out.String()
			}
			current = ruleStart
		default:
			out.WriteByte(ch)
			current++
		}
	}

	return out.String()
}

func findMatchingBrace(css string, start int) int {
	depth := 1
	for i := start + 1; i < len(css); i++ {
		ch := css[i]
		if ch == '\\' && i+1 < len(css) {
			i++
			continue
		}
		if ch == '{' {
			depth++
		} else if ch == '}' {
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}

func findMatchingSquare(css string, start int) int {
	depth := 1
	for i := start + 1; i < len(css); i++ {
		ch := css[i]
		if ch == '\\' && i+1 < len(css) {
			i++
			continue
		}
		if ch == '[' {
			depth++
		} else if ch == ']' {
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}

func indexSelectorSeparator(css string, start int) int {
	for i := start; i < len(css); i++ {
		switch css[i] {
		case ' ', '*', '>', '+', ',', ';', ':', '[', '{', ']':
			return i
		}
	}
	return -1
}

func indexSelectorEnd(css string, start int) int {
	for i := start; i < len(css); i++ {
		if css[i] == ',' || css[i] == '{' {
			return i
		}
	}
	return -1
}

func isSelectorStart(ch byte) bool {
	return isAlpha(ch) || ch == '.' || ch == '#' || ch == '*' || ch == '\\' || ch == ':' || ch == '['
}

func isAlpha(ch byte) bool {
	return (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z')
}

func hasNamespacePrefix(css string, start int, out *strings.Builder) bool {
	for i := start; i < len(css); i++ {
		c := css[i]
		if isAlpha(c) || (c >= '0' && c <= '9') || c == '_' || c == '-' || (c == '*' && i == start) {
			continue
		}
		if c == '|' {
			out.WriteString(css[start : i+1])
			return true
		}
		break
	}
	return false
}
