package live

import (
	"strings"
)

// Op is one VDOM patch operation. The bridge applies them in order
// against the live DOM rooted at the session's mount point.
//
//	{op: "replace", path: "0/2",       html: "<p>...</p>"}
//	{op: "text",    path: "0/1/0",     value: "Count: 4"}
//	{op: "attr",    path: "0/3",       attrs: {class: "active"}}
//	{op: "remove",  path: "0/4"}
//	{op: "insert",  path: "0/4",       html: "<li>...</li>"}
//	{op: "root",    html: "<...>" }
//
// Path is a slash-separated list of child indices from the mount
// point. The empty path "" means the mount root.
type Op struct {
	Op    string            `json:"op"`
	Path  string            `json:"path,omitempty"`
	Value string            `json:"value,omitempty"`
	HTML  string            `json:"html,omitempty"`
	Attrs map[string]string `json:"attrs,omitempty"`
}

// Diff produces an ordered list of Ops that turn `prev` HTML into
// `next` HTML. Both inputs are HTML fragments (the page body produced
// by the server-side renderer). The algorithm is structural, not
// textual: it parses each input into a small tree, then walks both in
// lock-step.
//
// The implementation favours correctness and predictability over peak
// efficiency. Two inputs that differ only in whitespace will produce
// minimal Ops; two inputs whose root structure differs will produce a
// single "root" Op carrying the full new HTML. Sub-trees that match
// shape but differ in attributes or text emit narrow Ops.
func Diff(prev, next string) []Op {
	if prev == next {
		return nil
	}
	pTree, pErr := parseTree(prev)
	nTree, nErr := parseTree(next)
	if pErr != nil || nErr != nil {
		return []Op{{Op: "root", HTML: next}}
	}
	var ops []Op
	diffNodes(pTree, nTree, "", &ops)
	if len(ops) == 0 && prev != next {
		// Structural match but text-only outside what we tracked.
		return []Op{{Op: "root", HTML: next}}
	}
	return ops
}

func diffNodes(a, b *node, path string, ops *[]Op) {
	if a == nil && b == nil {
		return
	}
	if a == nil {
		*ops = append(*ops, Op{Op: "insert", Path: path, HTML: b.render()})
		return
	}
	if b == nil {
		*ops = append(*ops, Op{Op: "remove", Path: path})
		return
	}
	if a.kind != b.kind || a.tag != b.tag {
		*ops = append(*ops, Op{Op: "replace", Path: path, HTML: b.render()})
		return
	}
	if a.kind == nodeText {
		if a.text != b.text {
			*ops = append(*ops, Op{Op: "text", Path: path, Value: b.text})
		}
		return
	}
	if !attrsEqual(a.attrs, b.attrs) {
		*ops = append(*ops, Op{Op: "attr", Path: path, Attrs: b.attrs})
	}
	la, lb := len(a.children), len(b.children)
	if la == lb {
		for i := 0; i < la; i++ {
			child := joinPath(path, i)
			diffNodes(a.children[i], b.children[i], child, ops)
		}
		return
	}
	// Length differs — emit replace at the parent. A future revision
	// can do keyed reconciliation; for now this is the predictable
	// fallback.
	*ops = append(*ops, Op{Op: "replace", Path: path, HTML: b.render()})
}

func joinPath(base string, idx int) string {
	if base == "" {
		return itoaPath(idx)
	}
	return base + "/" + itoaPath(idx)
}

func itoaPath(n int) string {
	if n == 0 {
		return "0"
	}
	var b [12]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}

func attrsEqual(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if b[k] != v {
			return false
		}
	}
	return true
}

// ---------------------------------------------------------------------
// Minimal HTML tree parser — element tags, attributes and text only.
// ---------------------------------------------------------------------

type nodeKind int

const (
	nodeElement nodeKind = iota
	nodeText
)

type node struct {
	kind     nodeKind
	tag      string
	attrs    map[string]string
	text     string
	children []*node
	raw      string // original source slice
}

// parseTree wraps the input in a synthetic <root> element and returns
// its children. It is not a full HTML5 parser — comments, CDATA, and
// rare constructs are passed through as text. Sufficient for diffing
// the output of the SSR renderer which produces well-formed HTML.
func parseTree(html string) (*node, error) {
	p := &htmlParser{src: html}
	root := &node{kind: nodeElement, tag: "root", attrs: map[string]string{}}
	for !p.eof() {
		c := p.parseChild()
		if c != nil {
			root.children = append(root.children, c)
		}
	}
	return root, nil
}

type htmlParser struct {
	src string
	i   int
}

func (p *htmlParser) eof() bool { return p.i >= len(p.src) }

func (p *htmlParser) parseChild() *node {
	if p.eof() {
		return nil
	}
	if p.src[p.i] == '<' && p.i+1 < len(p.src) {
		next := p.src[p.i+1]
		if next == '/' {
			// stray closing tag — consume to '>'
			end := strings.IndexByte(p.src[p.i:], '>')
			if end < 0 {
				p.i = len(p.src)
				return nil
			}
			p.i += end + 1
			return nil
		}
		if next == '!' || next == '?' {
			end := strings.IndexByte(p.src[p.i:], '>')
			if end < 0 {
				p.i = len(p.src)
				return nil
			}
			text := p.src[p.i : p.i+end+1]
			p.i += end + 1
			return &node{kind: nodeText, text: text, raw: text}
		}
		return p.parseElement()
	}
	return p.parseText()
}

func (p *htmlParser) parseText() *node {
	start := p.i
	for !p.eof() && p.src[p.i] != '<' {
		p.i++
	}
	text := p.src[start:p.i]
	if strings.TrimSpace(text) == "" {
		return nil
	}
	return &node{kind: nodeText, text: text, raw: text}
}

func (p *htmlParser) parseElement() *node {
	start := p.i
	p.i++ // skip <
	tagStart := p.i
	for !p.eof() && p.src[p.i] != ' ' && p.src[p.i] != '>' && p.src[p.i] != '/' && p.src[p.i] != '\t' && p.src[p.i] != '\n' {
		p.i++
	}
	tag := p.src[tagStart:p.i]

	attrs := map[string]string{}
	for !p.eof() && p.src[p.i] != '>' && p.src[p.i] != '/' {
		// skip whitespace
		for !p.eof() && (p.src[p.i] == ' ' || p.src[p.i] == '\t' || p.src[p.i] == '\n') {
			p.i++
		}
		if p.eof() || p.src[p.i] == '>' || p.src[p.i] == '/' {
			break
		}
		nameStart := p.i
		for !p.eof() && p.src[p.i] != '=' && p.src[p.i] != ' ' && p.src[p.i] != '>' && p.src[p.i] != '/' {
			p.i++
		}
		name := p.src[nameStart:p.i]
		val := ""
		if !p.eof() && p.src[p.i] == '=' {
			p.i++
			if !p.eof() && (p.src[p.i] == '"' || p.src[p.i] == '\'') {
				quote := p.src[p.i]
				p.i++
				vs := p.i
				for !p.eof() && p.src[p.i] != quote {
					p.i++
				}
				val = p.src[vs:p.i]
				if !p.eof() {
					p.i++
				}
			} else {
				vs := p.i
				for !p.eof() && p.src[p.i] != ' ' && p.src[p.i] != '>' && p.src[p.i] != '/' {
					p.i++
				}
				val = p.src[vs:p.i]
			}
		}
		if name != "" {
			attrs[name] = val
		}
	}

	selfClose := false
	if !p.eof() && p.src[p.i] == '/' {
		selfClose = true
		p.i++
	}
	if !p.eof() && p.src[p.i] == '>' {
		p.i++
	}

	n := &node{kind: nodeElement, tag: tag, attrs: attrs}
	if selfClose || isVoidElement(tag) {
		n.raw = p.src[start:p.i]
		return n
	}

	closer := "</" + tag + ">"
	for !p.eof() {
		if strings.HasPrefix(p.src[p.i:], closer) {
			p.i += len(closer)
			break
		}
		c := p.parseChild()
		if c != nil {
			n.children = append(n.children, c)
		}
		if p.eof() {
			break
		}
	}
	n.raw = p.src[start:p.i]
	return n
}

func isVoidElement(tag string) bool {
	switch tag {
	case "area", "base", "br", "col", "embed", "hr", "img", "input",
		"link", "meta", "param", "source", "track", "wbr":
		return true
	}
	return false
}

// render reconstructs HTML from a node — used when a diff Op needs to
// emit a fresh HTML payload (replace / insert).
func (n *node) render() string {
	if n.raw != "" {
		return n.raw
	}
	if n.kind == nodeText {
		return n.text
	}
	var sb strings.Builder
	sb.WriteByte('<')
	sb.WriteString(n.tag)
	for k, v := range n.attrs {
		sb.WriteByte(' ')
		sb.WriteString(k)
		sb.WriteByte('=')
		sb.WriteByte('"')
		sb.WriteString(v)
		sb.WriteByte('"')
	}
	if isVoidElement(n.tag) {
		sb.WriteString(" />")
		return sb.String()
	}
	sb.WriteByte('>')
	for _, c := range n.children {
		sb.WriteString(c.render())
	}
	sb.WriteString("</")
	sb.WriteString(n.tag)
	sb.WriteByte('>')
	return sb.String()
}
