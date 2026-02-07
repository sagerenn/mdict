package mdict

import (
	"bytes"
	"encoding/binary"
	"io"
	"regexp"
	"strings"
	"unicode/utf16"
	"unicode/utf8"

	"github.com/sagerenn/mdict/internal/dict"

	"golang.org/x/text/encoding/htmlindex"
	"golang.org/x/text/transform"
)

var (
	fontURLRe  = regexp.MustCompile(`(?i)url\s*\(\s*\"(.*?)\"\s*\)`)
	styleTagRe = regexp.MustCompile(`(?is)(<style[^>]*>)([\s\S]*?)(</style>)`)
)

func isCSSFile(name string) bool {
	return strings.HasSuffix(strings.ToLower(name), ".css")
}

func processCSS(data []byte, encoding, dictID string) []byte {
	if len(data) == 0 {
		return data
	}
	css := decodeCSS(data, encoding)
	css = dict.RewriteCSSLinks(css, dictID)
	css = dict.IsolateCSS(css, dictID, "")
	return []byte(css)
}

func rewriteFontLinksInHTML(html, dictID string) string {
	if html == "" {
		return html
	}
	return styleTagRe.ReplaceAllStringFunc(html, func(m string) string {
		sub := styleTagRe.FindStringSubmatch(m)
		if len(sub) != 4 {
			return m
		}
		content := rewriteFontLinks(sub[2], dictID)
		return sub[1] + content + sub[3]
	})
}

func isolateStyleCSSInHTML(html, dictID string) string {
	if html == "" || !strings.Contains(strings.ToLower(html), "<style") {
		return html
	}
	return styleTagRe.ReplaceAllStringFunc(html, func(m string) string {
		sub := styleTagRe.FindStringSubmatch(m)
		if len(sub) != 4 {
			return m
		}
		content := dict.IsolateCSS(sub[2], dictID, "")
		return sub[1] + content + sub[3]
	})
}

func rewriteFontLinks(css, dictID string) string {
	if css == "" {
		return css
	}
	var out strings.Builder
	last := 0
	for _, loc := range fontURLRe.FindAllStringSubmatchIndex(css, -1) {
		if len(loc) < 4 {
			continue
		}
		out.WriteString(css[last:loc[0]])
		url := css[loc[2]:loc[3]]
		if strings.Contains(url, ":") {
			out.WriteString(css[loc[0]:loc[1]])
		} else {
			out.WriteString(`url("` + dict.ResourceURL(dictID, url) + `")`)
		}
		last = loc[1]
	}
	out.WriteString(css[last:])
	return out.String()
}

func decodeCSS(data []byte, fallbackEncoding string) string {
	if len(data) == 0 {
		return ""
	}
	if len(data) >= 3 && data[0] == 0xEF && data[1] == 0xBB && data[2] == 0xBF {
		return string(data[3:])
	}
	if len(data) >= 4 && data[0] == 0x00 && data[1] == 0x00 && data[2] == 0xFE && data[3] == 0xFF {
		return decodeUTF32(data[4:], true)
	}
	if len(data) >= 4 && data[0] == 0xFF && data[1] == 0xFE && data[2] == 0x00 && data[3] == 0x00 {
		return decodeUTF32(data[4:], false)
	}
	if len(data) >= 2 && data[0] == 0xFE && data[1] == 0xFF {
		return decodeUTF16(data[2:], true)
	}
	if len(data) >= 2 && data[0] == 0xFF && data[1] == 0xFE {
		return decodeUTF16(data[2:], false)
	}
	if utf8.Valid(data) {
		return string(data)
	}
	if strings.HasPrefix(strings.ToUpper(fallbackEncoding), "UTF-16") {
		return decodeUTF16(data, strings.Contains(strings.ToUpper(fallbackEncoding), "BE"))
	}
	if fallbackEncoding != "" {
		if decoded, ok := decodeWithEncoding(fallbackEncoding, data); ok {
			return decoded
		}
	}
	return string(data)
}

func decodeUTF16(data []byte, bigEndian bool) string {
	if len(data) < 2 {
		return ""
	}
	if len(data)%2 == 1 {
		data = data[:len(data)-1]
	}
	u16 := make([]uint16, len(data)/2)
	for i := 0; i < len(u16); i++ {
		if bigEndian {
			u16[i] = binary.BigEndian.Uint16(data[i*2 : i*2+2])
		} else {
			u16[i] = binary.LittleEndian.Uint16(data[i*2 : i*2+2])
		}
	}
	return string(utf16.Decode(u16))
}

func decodeUTF32(data []byte, bigEndian bool) string {
	if len(data) < 4 {
		return ""
	}
	if len(data)%4 != 0 {
		data = data[:len(data)/4*4]
	}
	runes := make([]rune, 0, len(data)/4)
	for i := 0; i+3 < len(data); i += 4 {
		var v uint32
		if bigEndian {
			v = uint32(data[i])<<24 | uint32(data[i+1])<<16 | uint32(data[i+2])<<8 | uint32(data[i+3])
		} else {
			v = uint32(data[i+3])<<24 | uint32(data[i+2])<<16 | uint32(data[i+1])<<8 | uint32(data[i])
		}
		if v == 0 {
			continue
		}
		runes = append(runes, rune(v))
	}
	return string(runes)
}

func decodeWithEncoding(label string, data []byte) (string, bool) {
	enc, err := htmlindex.Get(label)
	if err != nil {
		return "", false
	}
	reader := transform.NewReader(bytes.NewReader(data), enc.NewDecoder())
	decoded, err := io.ReadAll(reader)
	if err != nil {
		return "", false
	}
	return string(decoded), true
}
