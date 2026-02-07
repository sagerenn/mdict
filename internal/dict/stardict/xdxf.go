package stardict

import (
	"encoding/xml"
	"errors"
	"html"
	"io"
	"strings"

	gd "github.com/sagerenn/mdict/internal/dict"
)

type xdxfNodeType int

const (
	xdxfElement xdxfNodeType = iota
	xdxfText
)

type xdxfAttr struct {
	key   string
	value string
}

type xdxfNode struct {
	typ      xdxfNodeType
	data     string
	attr     []xdxfAttr
	children []*xdxfNode
}

func xdxfToHTML(input, dictID string) string {
	if input == "" {
		return ""
	}

	converted := xdxfNormalizeInput(input)
	wrapped := `<root><div class="sdct_x">` + converted + `</div></root>`

	root, err := xdxfParse(wrapped)
	if err != nil || root == nil {
		return input
	}

	// Find wrapper div.
	var wrapper *xdxfNode
	for _, child := range root.children {
		if child.typ == xdxfElement && child.data == "div" {
			wrapper = child
			break
		}
	}
	if wrapper == nil {
		return input
	}

	ctx := xdxfContext{dictID: dictID}
	xdxfTransform(wrapper, &ctx)

	var b strings.Builder
	xdxfRender(&b, wrapper)
	return b.String()
}

type xdxfContext struct {
	dictID string
}

func xdxfNormalizeInput(in string) string {
	var b strings.Builder
	b.Grow(len(in))
	afterEol := false
	for i := 0; i < len(in); i++ {
		ch := in[i]
		switch ch {
		case '\n':
			afterEol = true
			b.WriteString("<br/>")
		case '\r':
			// skip
		case ' ':
			if afterEol {
				b.WriteString("&#160;")
			} else {
				b.WriteByte(ch)
			}
		default:
			b.WriteByte(ch)
			afterEol = false
		}
	}
	return b.String()
}

func xdxfParse(input string) (*xdxfNode, error) {
	dec := xml.NewDecoder(strings.NewReader(input))
	dec.Strict = false
	root := &xdxfNode{typ: xdxfElement, data: "root"}
	stack := []*xdxfNode{root}

	for {
		tok, err := dec.Token()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			node := &xdxfNode{
				typ:  xdxfElement,
				data: strings.ToLower(t.Name.Local),
			}
			for _, a := range t.Attr {
				key := strings.ToLower(a.Name.Local)
				if a.Name.Space != "" {
					key = strings.ToLower(a.Name.Space + ":" + a.Name.Local)
				}
				node.attr = append(node.attr, xdxfAttr{key: key, value: a.Value})
			}
			parent := stack[len(stack)-1]
			parent.children = append(parent.children, node)
			stack = append(stack, node)
		case xml.EndElement:
			if len(stack) > 1 {
				stack = stack[:len(stack)-1]
			}
		case xml.CharData:
			data := string(t)
			if data != "" {
				parent := stack[len(stack)-1]
				parent.children = append(parent.children, &xdxfNode{typ: xdxfText, data: data})
			}
		}
	}
	return root, nil
}

func xdxfTransform(node *xdxfNode, ctx *xdxfContext) {
	if node == nil || node.typ != xdxfElement {
		return
	}
	for i := 0; i < len(node.children); i++ {
		child := node.children[i]
		if child.typ == xdxfElement {
			xdxfTransform(child, ctx)
			xdxfApply(child, node, i, ctx)
		}
	}
}

func xdxfApply(n, parent *xdxfNode, idx int, ctx *xdxfContext) {
	switch n.data {
	case "ex":
		n.data = "span"
		n.setAttr("class", "xdxf_ex_old")
		author, _ := n.getAttr("author")
		source, _ := n.getAttr("source")
		if (author != "" || source != "") && xdxfHasContent(n) {
			text := strings.TrimSpace(author)
			if source != "" {
				if text != "" {
					text += ", "
				}
				text += source
			}
			text = strings.TrimSpace(text)
			if text != "" {
				src := &xdxfNode{typ: xdxfElement, data: "span"}
				src.setAttr("class", "xdxf_ex_source")
				src.children = append(src.children, &xdxfNode{typ: xdxfText, data: text})
				n.children = append(n.children, src)
			}
		}
	case "ex_orig":
		n.data = "span"
		n.setAttr("class", "xdxf_ex_orig")
	case "ex_tran":
		n.data = "span"
		n.setAttr("class", "xdxf_ex_tran")
	case "mrkd":
		n.data = "span"
		n.setAttr("class", "xdxf_ex_markd")
	case "k":
		n.data = "span"
		n.setAttr("class", "xdxf_k")
	case "opt":
		n.data = "span"
		n.setAttr("class", "xdxf_opt")
	case "kref":
		text := strings.TrimSpace(xdxfTextContent(n))
		n.data = "a"
		n.setAttr("class", "xdxf_kref")
		href := gd.EntryURL(ctx.dictID, text)
		if idref, ok := n.getAttr("idref"); ok && idref != "" {
			href += "#" + idref
		}
		n.setAttr("href", href)
		if kcmt, ok := n.getAttr("kcmt"); ok && kcmt != "" && parent != nil {
			comment := &xdxfNode{typ: xdxfText, data: " " + kcmt}
			parent.children = append(parent.children[:idx+1], append([]*xdxfNode{comment}, parent.children[idx+1:]...)...)
		}
	case "iref":
		ref := ""
		if v, ok := n.getAttr("href"); ok {
			ref = v
		}
		if ref == "" {
			ref = strings.TrimSpace(xdxfTextContent(n))
		}
		n.data = "a"
		if ref != "" {
			n.setAttr("href", ref)
		}
	case "abr", "abbr":
		n.data = "span"
		n.setAttr("class", "xdxf_abbr")
	case "dtrn":
		n.data = "span"
		n.setAttr("class", "xdxf_dtrn")
	case "c":
		n.data = "span"
		if color, ok := n.getAttr("c"); ok && color != "" {
			n.setAttr("style", "color:"+color)
		} else {
			n.setAttr("style", "color:blue")
		}
		n.removeAttr("c")
	case "co":
		n.data = "span"
		n.setAttr("class", "xdxf_co_old")
	case "gr", "pos", "tense":
		n.data = "span"
		n.setAttr("class", "xdxf_gr_old")
	case "tr":
		n.data = "span"
		n.setAttr("class", "xdxf_tr_old")
	case "img":
		xdxfRewriteAttr(n, "src", ctx.dictID)
		xdxfRewriteAttr(n, "losrc", ctx.dictID)
		xdxfRewriteAttr(n, "hisrc", ctx.dictID)
	case "rref":
		if _, ok := n.getAttr("start"); ok {
			n.data = "span"
			n.setAttr("class", "xdxf_rref")
			return
		}
		filename := strings.TrimSpace(xdxfTextContent(n))
		if filename == "" {
			n.data = "span"
			n.setAttr("class", "xdxf_rref")
			return
		}
		switch {
		case isImageFile(filename):
			n.data = "img"
			n.children = nil
			n.setAttr("src", gd.ResourceURL(ctx.dictID, filename))
			n.setAttr("alt", filename)
		case isSoundFile(filename):
			n.data = "audio"
			n.children = nil
			n.setAttr("controls", "controls")
			n.setAttr("src", gd.ResourceURL(ctx.dictID, filename))
		default:
			n.data = "span"
			n.setAttr("class", "xdxf_rref")
		}
	}
}

func xdxfHasContent(n *xdxfNode) bool {
	return strings.TrimSpace(xdxfTextContent(n)) != ""
}

func xdxfTextContent(n *xdxfNode) string {
	var b strings.Builder
	xdxfCollectText(&b, n)
	return b.String()
}

func xdxfCollectText(b *strings.Builder, n *xdxfNode) {
	if n == nil {
		return
	}
	if n.typ == xdxfText {
		b.WriteString(n.data)
		return
	}
	for _, c := range n.children {
		xdxfCollectText(b, c)
	}
}

func xdxfRewriteAttr(n *xdxfNode, key, dictID string) {
	if n == nil {
		return
	}
	val, ok := n.getAttr(key)
	if !ok || val == "" {
		return
	}
	if xdxfIsExternalURL(val) {
		return
	}
	n.setAttr(key, gd.ResourceURL(dictID, val))
}

func xdxfIsExternalURL(val string) bool {
	u := strings.TrimSpace(val)
	if u == "" {
		return true
	}
	lower := strings.ToLower(u)
	if strings.HasPrefix(lower, "#") {
		return true
	}
	if strings.Contains(lower, "://") {
		return true
	}
	if strings.HasPrefix(lower, "data:") || strings.HasPrefix(lower, "mailto:") || strings.HasPrefix(lower, "bres:") ||
		strings.HasPrefix(lower, "gdau:") || strings.HasPrefix(lower, "gdlookup:") || strings.HasPrefix(lower, "qrc:") {
		return true
	}
	return false
}

func (n *xdxfNode) getAttr(key string) (string, bool) {
	key = strings.ToLower(key)
	for _, a := range n.attr {
		if a.key == key {
			return a.value, true
		}
	}
	return "", false
}

func (n *xdxfNode) setAttr(key, value string) {
	key = strings.ToLower(key)
	for i := range n.attr {
		if n.attr[i].key == key {
			n.attr[i].value = value
			return
		}
	}
	n.attr = append(n.attr, xdxfAttr{key: key, value: value})
}

func (n *xdxfNode) removeAttr(key string) {
	key = strings.ToLower(key)
	out := n.attr[:0]
	for _, a := range n.attr {
		if a.key != key {
			out = append(out, a)
		}
	}
	n.attr = out
}

func xdxfRender(b *strings.Builder, n *xdxfNode) {
	if n == nil {
		return
	}
	switch n.typ {
	case xdxfText:
		b.WriteString(html.EscapeString(n.data))
	case xdxfElement:
		b.WriteByte('<')
		b.WriteString(n.data)
		if len(n.attr) > 0 {
			for _, a := range n.attr {
				if a.value == "" {
					continue
				}
				b.WriteByte(' ')
				b.WriteString(a.key)
				b.WriteString(`="`)
				b.WriteString(html.EscapeString(a.value))
				b.WriteByte('"')
			}
		}
		b.WriteByte('>')
		if !xdxfIsVoidTag(n.data) {
			for _, c := range n.children {
				xdxfRender(b, c)
			}
			b.WriteString("</")
			b.WriteString(n.data)
			b.WriteByte('>')
		}
	}
}

func xdxfIsVoidTag(tag string) bool {
	switch tag {
	case "br", "hr", "img", "meta", "link", "source", "input", "area", "base", "col", "embed", "param", "track", "wbr":
		return true
	default:
		return false
	}
}
