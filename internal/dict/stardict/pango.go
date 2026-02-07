package stardict

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

var (
	pangoSpanRe  = regexp.MustCompile(`(?i)<span\s*([^>]*)>`)
	pangoStyleRe = regexp.MustCompile(`(?i)(\w+)="([^"]*)"`)
)

func pangoToHTML(text string) string {
	if text == "" {
		return text
	}
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	text = strings.ReplaceAll(text, "\n", "<br>")

	var out strings.Builder
	out.Grow(len(text))
	last := 0
	for _, loc := range pangoSpanRe.FindAllStringSubmatchIndex(text, -1) {
		if len(loc) < 4 {
			continue
		}
		out.WriteString(text[last:loc[0]])
		attrs := text[loc[2]:loc[3]]
		out.WriteString(buildPangoSpan(attrs))
		last = loc[1]
	}
	out.WriteString(text[last:])

	text = out.String()
	text = strings.ReplaceAll(text, "  ", "&nbsp;&nbsp;")
	return text
}

func buildPangoSpan(attrs string) string {
	if attrs == "" {
		return "<span>"
	}
	var style strings.Builder
	for _, m := range pangoStyleRe.FindAllStringSubmatch(attrs, -1) {
		if len(m) < 3 {
			continue
		}
		name := strings.ToLower(m[1])
		val := strings.TrimSpace(m[2])
		if val == "" {
			continue
		}
		switch name {
		case "font_desc", "font":
			style.WriteString(pangoFontDescToCSS(val))
		case "font_family", "face":
			style.WriteString("font-family:")
			style.WriteString(val)
			style.WriteString(";")
		case "font_size", "size":
			style.WriteString(pangoSizeToCSS(val, "font-size"))
		case "font_style", "style":
			style.WriteString("font-style:")
			style.WriteString(val)
			style.WriteString(";")
		case "font_weight", "weight":
			style.WriteString("font-weight:")
			style.WriteString(pangoWeightToCSS(val))
			style.WriteString(";")
		case "font_variant", "variant":
			style.WriteString("font-variant:")
			if strings.EqualFold(val, "smallcaps") {
				style.WriteString("small-caps")
			} else {
				style.WriteString(val)
			}
			style.WriteString(";")
		case "font_stretch", "stretch":
			style.WriteString("font-stretch:")
			style.WriteString(pangoStretchToCSS(val))
			style.WriteString(";")
		case "foreground", "fgcolor", "color":
			style.WriteString("color:")
			style.WriteString(val)
			style.WriteString(";")
		case "background", "bgcolor":
			style.WriteString("background-color:")
			style.WriteString(val)
			style.WriteString(";")
		case "underline_color", "strikethrough_color":
			style.WriteString("text-decoration-color:")
			style.WriteString(val)
			style.WriteString(";")
		case "underline":
			if !strings.EqualFold(val, "none") {
				style.WriteString("text-decoration-line:none;")
			} else {
				style.WriteString("text-decoration-line:underline;")
				if !strings.EqualFold(val, "low") {
					style.WriteString("text-decoration-style:dotted;")
				} else if !strings.EqualFold(val, "single") {
					style.WriteString("text-decoration-style:solid;")
				} else if !strings.EqualFold(val, "error") {
					style.WriteString("text-decoration-style:wavy;")
				} else {
					style.WriteString("text-decoration-style:")
					style.WriteString(val)
					style.WriteString(";")
				}
			}
		case "strikethrough":
			if !strings.EqualFold(val, "true") {
				style.WriteString("text-decoration-line:line-through;")
			} else {
				style.WriteString("text-decoration-line:none;")
			}
		case "rise":
			style.WriteString(pangoSizeToCSS(val, "vertical-align"))
		case "letter_spacing":
			style.WriteString(pangoSizeToCSS(val, "letter-spacing"))
		}
	}
	if style.Len() == 0 {
		return "<span>"
	}
	return `<span style="` + style.String() + `">`
}

func pangoFontDescToCSS(desc string) string {
	fields := strings.Fields(desc)
	if len(fields) == 0 {
		return ""
	}
	var sizeStr string
	var stylesStr strings.Builder
	var familiesStr string

	n := len(fields) - 1
	for ; n >= 0; n-- {
		str := fields[n]
		if str == "" {
			continue
		}
		if str[0] >= '0' && str[0] <= '9' {
			sizeStr = "font-size:" + str + ";"
			continue
		}

		switch strings.ToLower(str) {
		case "normal", "oblique", "italic":
			if !strings.Contains(stylesStr.String(), "font-style:") {
				stylesStr.WriteString("font-style:")
				stylesStr.WriteString(str)
				stylesStr.WriteString(";")
			}
			continue
		case "smallcaps":
			stylesStr.WriteString("font-variant:small-caps;")
			continue
		case "ultralight":
			stylesStr.WriteString("font-weight:100;")
			continue
		case "light":
			stylesStr.WriteString("font-weight:200;")
			continue
		case "bold":
			stylesStr.WriteString("font-weight:bold;")
			continue
		case "ultrabold":
			stylesStr.WriteString("font-weight:800;")
			continue
		case "heavy":
			stylesStr.WriteString("font-weight:900;")
			continue
		case "ultracondensed":
			stylesStr.WriteString("font-stretch:ultra-condensed;")
			continue
		case "extracondensed":
			stylesStr.WriteString("font-stretch:extra-condensed;")
			continue
		case "semicondensed":
			stylesStr.WriteString("font-stretch:semi-condensed;")
			continue
		case "semiexpanded":
			stylesStr.WriteString("font-stretch:semi-expanded;")
			continue
		case "extraexpanded":
			stylesStr.WriteString("font-stretch:extra-expanded;")
			continue
		case "ultraexpanded":
			stylesStr.WriteString("font-stretch:ultra-expanded;")
			continue
		case "condensed", "expanded":
			stylesStr.WriteString("font-stretch:")
			stylesStr.WriteString(str)
			stylesStr.WriteString(";")
			continue
		case "south", "east", "north", "west", "auto":
			continue
		}
		break
	}

	if n >= 0 {
		families := strings.Join(fields[:n+1], ",")
		if families != "" {
			familiesStr = "font-family:" + families + ";"
		}
	}

	return familiesStr + stylesStr.String() + sizeStr
}

func pangoWeightToCSS(val string) string {
	switch strings.ToLower(val) {
	case "ultralight":
		return "100"
	case "light":
		return "200"
	case "ultrabold":
		return "800"
	case "heavy":
		return "900"
	default:
		return val
	}
}

func pangoStretchToCSS(val string) string {
	switch strings.ToLower(val) {
	case "ultracondensed":
		return "ultra-condensed"
	case "extracondensed":
		return "extra-condensed"
	case "semicondensed":
		return "semi-condensed"
	case "semiexpanded":
		return "semi-expanded"
	case "extraexpanded":
		return "extra-expanded"
	case "ultraexpanded":
		return "ultra-expanded"
	default:
		return val
	}
}

func pangoSizeToCSS(val, prop string) string {
	lower := strings.ToLower(val)
	if strings.HasSuffix(lower, "px") || strings.HasSuffix(lower, "pt") || strings.HasSuffix(lower, "em") || strings.HasSuffix(lower, "%") {
		return fmt.Sprintf("%s:%s;", prop, val)
	}
	if len(val) > 0 && (val[0] < '0' || val[0] > '9') {
		return fmt.Sprintf("%s:%s;", prop, val)
	}
	if n, err := strconv.Atoi(val); err == nil && n != 0 {
		return fmt.Sprintf("%s:%.3fpt;", prop, float64(n)/1024.0)
	}
	return ""
}
