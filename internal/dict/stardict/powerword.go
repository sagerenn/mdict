package stardict

import (
	"encoding/xml"
	"errors"
	"io"
	"regexp"
	"strings"
)

var (
	pwTranslate = []struct {
		re   *regexp.Regexp
		repl string
	}{
		{regexp.MustCompile(`&[bB]\s*\{([^\{}&]+)\}`), "<B>$1</B>"},
		{regexp.MustCompile(`&[iI]\s*\{([^\{}&]+)\}`), "<I>$1</I>"},
		{regexp.MustCompile(`&[uU]\s*\{([^\{}&]+)\}`), "<U>$1</U>"},
		{regexp.MustCompile(`&[lL]\s*\{([^\{}&]+)\}`), `<SPAN style="color:#0000ff">$1</SPAN>`},
		{regexp.MustCompile(`&[2]\s*\{([^\{}&]+)\}`), `<SPAN style="color:#0000ff">$1</SPAN>`},
	}
	pwLeadingBrace = regexp.MustCompile(`&.\s*\{`)
)

func powerWordToHTML(data string) string {
	ss := `<div class="sdct_k">`
	texts, ok := powerWordTextNodes(data)
	if !ok {
		return ss + data + `</div>`
	}
	var b strings.Builder
	b.Grow(len(data) + 32)
	b.WriteString(ss)
	for _, s := range texts {
		s = translatePowerWord(s)
		b.WriteString(s)
		b.WriteString("<br>")
	}
	b.WriteString(`</div>`)
	return b.String()
}

func powerWordTextNodes(data string) ([]string, bool) {
	dec := xml.NewDecoder(strings.NewReader(data))
	dec.Strict = false
	var out []string
	for {
		tok, err := dec.Token()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, false
		}
		if cd, ok := tok.(xml.CharData); ok {
			out = append(out, string(cd))
		}
	}
	return out, true
}

func translatePowerWord(s string) string {
	prev := ""
	for s != prev {
		prev = s
		for _, t := range pwTranslate {
			s = t.re.ReplaceAllString(s, t.repl)
		}
	}
	s = pwLeadingBrace.ReplaceAllString(s, "")
	s = strings.ReplaceAll(s, "}", "")
	return s
}
