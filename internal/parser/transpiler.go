package parser

import (
	"fmt"
	"regexp"
	"strings"
)

// ComponentInfo contains transpiled component information
type ComponentInfo struct {
	PackageName   string   // Generated Go package name
	SourceCode    string   // Complete Go source code
	StateVars     []string // Detected state variables
	EventHandlers []string // Detected event handler names
}

// safeGoExprRx matches a restricted expression grammar that is safe to paste
// into generated Go source without enabling code-injection via crafted
// .gox templates. Allowed: identifiers, dot access, indexing, simple
// comparisons, numeric/string literals, boolean operators, parentheses.
// Rejected: statements, function bodies, semicolons, braces, backticks,
// new keyword usage such as `go`/`defer`/`func`.
var safeGoExprRx = regexp.MustCompile(`^[A-Za-z0-9_\.\[\]\(\),\s<>=!&|+\-*/%:"'\x60]*$`)

var disallowedExprTokens = []string{
	"{", "}", ";", "//", "/*", "*/", "\n", "\r",
	"func", "go ", "defer ", "import ", "package ",
	"recover(", "panic(", "os.", "exec.", "syscall.", "runtime.",
	"unsafe.", "reflect.", "cgo.",
}

// isSafeGoExpr reports whether `expr` is restricted enough to emit into a
// generated Go conditional or range target. Returns (cleaned, true) when
// safe. Call sites MUST fall back to a literal `false` / `nil` when unsafe.
func isSafeGoExpr(expr string) (string, bool) {
	trimmed := strings.TrimSpace(expr)
	if trimmed == "" {
		return "", false
	}
	if !safeGoExprRx.MatchString(trimmed) {
		return "", false
	}
	lowered := strings.ToLower(trimmed)
	for _, bad := range disallowedExprTokens {
		if strings.Contains(lowered, bad) {
			return "", false
		}
	}
	return trimmed, true
}

// unsafeURLSchemeRx matches href values that should never be emitted into
// SSR HTML attributes because they execute arbitrary scripts when followed.
var unsafeURLSchemeRx = regexp.MustCompile(`(?i)^\s*(javascript|data|vbscript|file):`)

// sanitizeHref returns href unchanged for safe values and an empty string
// when the scheme is dangerous. Called before attribute emission.
func sanitizeHref(href string) string {
	if unsafeURLSchemeRx.MatchString(href) {
		return "#"
	}
	return href
}

// identRx matches a bare Go identifier used as a loop variable name.
var identRx = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// tagRx matches an HTML tag name suitable for emission as a wrapper.
var tagRx = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9\-]*$`)

func isSafeIdent(s string) bool {
	return identRx.MatchString(s)
}

func isSafeTagName(s string) bool {
	return tagRx.MatchString(s)
}

var (
	// Regex for variable interpolation {varName}
	varPattern = regexp.MustCompile(`\{([a-zA-Z_][a-zA-Z0-9_]*)\}`)
	// Regex for event attributes @event="handler()"
	eventPattern = regexp.MustCompile(`@(\w+)="(\w+)\(\)"`)
	// Regex for HTML attributes key="value" or key='value'
	attrPattern = regexp.MustCompile(`(\S+)=["']([^"']*)["']`)
	// Regex to strip content inside <code> and <pre> tags before variable extraction
	literalTagRx = regexp.MustCompile(`(?s)<(?:code|pre)\b[^>]*>.*?</(?:code|pre)>`)

	// Pre-compiled regexes for cleanGoCode
	revalidateRx = regexp.MustCompile(`^export\s+const\s+revalidate\s*=\s*\d+`)
	layoutRx     = regexp.MustCompile(`^layout\s*=\s*"[^"]+"`)
	shortDeclRx  = regexp.MustCompile(`^(\w+(?:,\s*\w+)*)\s*:=\s*(.+)$`)
)

// --- Internal HTML tree for rendering to VNode code ---

type htmlNode struct {
	tag         string
	attrs       map[string]string
	events      map[string]string // @click → handlerName
	text        string
	children    []*htmlNode
	isText      bool
	isSelfClose bool
}

func (n *htmlNode) addChild(child *htmlNode) {
	n.children = append(n.children, child)
}

// Transpile converts a File into Go source code for WASM (CSR mode).
// Generates VNode-based code (core.H) that the reconciler can diff efficiently.
func Transpile(f *File) *ComponentInfo {
	info := &ComponentInfo{
		PackageName: f.PackageName(),
	}

	info.StateVars = extractStateVars(f.HTMLTemplate)
	info.EventHandlers = extractEventHandlers(f.HTMLTemplate)
	info.SourceCode = generateSource(f, info)

	return info
}

// extractStateVars finds all {varName} variables in the template,
// skipping content inside <code> and <pre> tags.
func extractStateVars(template string) []string {
	cleaned := literalTagRx.ReplaceAllString(template, "")
	matches := varPattern.FindAllStringSubmatch(cleaned, -1)
	seen := make(map[string]bool)
	var vars []string
	for _, match := range matches {
		name := match[1]
		if !seen[name] {
			seen[name] = true
			vars = append(vars, name)
		}
	}
	return vars
}

// extractEventHandlers finds all @event="handler()" in the template
func extractEventHandlers(template string) []string {
	matches := eventPattern.FindAllStringSubmatch(template, -1)
	seen := make(map[string]bool)
	var handlers []string
	for _, match := range matches {
		name := match[2]
		if !seen[name] {
			seen[name] = true
			handlers = append(handlers, name)
		}
	}
	return handlers
}

// cleanGoCode removes Nguyen.go frontmatter pseudo-syntax (layout=, export const metadata/geo/revalidate)
// and returns only the Go code that TinyGo can compile.
// It also converts top-level := short declarations to var declarations, since :=
// is only valid inside functions.
func cleanGoCode(src string) string {
	if src == "" {
		return ""
	}

	var out []string
	lines := strings.Split(src, "\n")
	inExportBlock := false
	exportBlockDepth := 0
	inFuncBody := false
	funcBodyDepth := 0

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			out = append(out, line)
			continue
		}

		// Skip single-line exports: export const revalidate = 60
		if revalidateRx.MatchString(trimmed) {
			continue
		}

		// Skip export const metadata/geo blocks
		if !inExportBlock && strings.HasPrefix(trimmed, "export const") {
			if strings.Contains(trimmed, "metadata") || strings.Contains(trimmed, "geo") {
				if strings.Contains(trimmed, "{") {
					exportBlockDepth = strings.Count(trimmed, "{") - strings.Count(trimmed, "}")
					if exportBlockDepth <= 0 {
						continue // single line block
					}
					inExportBlock = true
				} else {
					inExportBlock = true
					exportBlockDepth = 0
				}
				continue
			}
		}

		if inExportBlock {
			exportBlockDepth += strings.Count(trimmed, "{") - strings.Count(trimmed, "}")
			if exportBlockDepth <= 0 {
				inExportBlock = false
				exportBlockDepth = 0
			}
			continue
		}

		// Skip layout assignment line (not valid Go)
		if layoutRx.MatchString(trimmed) {
			continue
		}

		// Track function bodies — don't convert := inside functions
		if !inFuncBody && strings.HasPrefix(trimmed, "func ") {
			if strings.Contains(trimmed, "{") {
				funcBodyDepth = strings.Count(trimmed, "{") - strings.Count(trimmed, "}")
				if funcBodyDepth > 0 {
					inFuncBody = true
				}
			}
			out = append(out, line)
			continue
		}

		if inFuncBody {
			funcBodyDepth += strings.Count(trimmed, "{") - strings.Count(trimmed, "}")
			if funcBodyDepth <= 0 {
				inFuncBody = false
				funcBodyDepth = 0
			}
			out = append(out, line)
			continue
		}

		// At package level (not inside a function): convert := to var.
		// Single-var := with multi-return functions (e.g. UseGlobalState) needs a blank var.
		if m := shortDeclRx.FindStringSubmatch(trimmed); m != nil {
			lhs := m[1]
			rhs := m[2]
			if !strings.Contains(lhs, ",") && strings.Contains(rhs, "UseGlobalState") {
				lhs = lhs + ", _"
			}
			indent := line[:len(line)-len(strings.TrimLeft(line, " \t"))]
			line = indent + "var " + lhs + " = " + rhs
		}

		// Skip standalone import lines (the generated file already imports core)
		if strings.HasPrefix(trimmed, "import ") {
			continue
		}

		out = append(out, line)
	}

	return strings.TrimSpace(strings.Join(out, "\n"))
}

// generateSource creates complete Go source from File (CSR/WASM mode)
func generateSource(f *File, info *ComponentInfo) string {
	var sb strings.Builder

	sb.WriteString("// Auto-generated by Nguyen.go - DO NOT EDIT\n")
	sb.WriteString("// Source: " + f.Path + "\n")
	sb.WriteString("package main\n\n")

	goCode := cleanGoCode(f.GoCode)

	// Only import what the generated code actually uses to avoid unused-import errors.
	needsFmt := len(info.StateVars) > 0
	needsJS := len(info.EventHandlers) > 0
	if !needsFmt && strings.Contains(goCode, "fmt.") {
		needsFmt = true
	}
	if !needsJS && strings.Contains(goCode, "js.") {
		needsJS = true
	}

	sb.WriteString("import (\n")
	if needsFmt {
		sb.WriteString("\t\"fmt\"\n")
	}
	if needsJS {
		sb.WriteString("\t\"syscall/js\"\n")
	}
	sb.WriteString("\t\"github.com/dev2k6/Nguyen.go/pkg/core\"\n")
	sb.WriteString(")\n\n")

	if goCode != "" {
		sb.WriteString("// --- User code from frontmatter ---\n")
		sb.WriteString(goCode)
		sb.WriteString("\n\n")
	}

	sb.WriteString("// Render returns the VNode tree for this component.\n")
	sb.WriteString("func Render() *core.VNode {\n")

	// Parse HTML into tree and emit VNode code
	tree := parseHTML(f.HTMLTemplate)
	if tree != nil {
		// Phase 1: collect and emit event handler registrations
		events := collectEvents(tree)
		for _, ev := range events {
			sb.WriteString("\tcore.RegisterEventHandler(\"" + ev.key + "\", func(e js.Value) {\n")
			sb.WriteString("\t\t" + ev.handler + "()\n")
			sb.WriteString("\t})\n")
		}
		if len(events) > 0 {
			sb.WriteString("\n")
		}

		// Phase 2: emit VNode tree with return
		sb.WriteString("\treturn ")
		emitVNodeExpr(&sb, tree, "\t")
		sb.WriteString("\n")
	} else {
		sb.WriteString("\treturn nil\n")
	}

	sb.WriteString("}\n\n")
	// TinyGo wasm target requires a main() entry point
	sb.WriteString("func main() {}\n")
	return sb.String()
}

type eventReg struct {
	key     string // registry key, e.g. "nguyen_increment_click"
	handler string // handler function name, e.g. "increment"
}

// collectEvents walks the tree and collects event registrations, mutating attrs to add data-nguyen-* keys
func collectEvents(node *htmlNode) []eventReg {
	if node == nil || node.isText {
		return nil
	}
	var events []eventReg
	for evName, handlerName := range node.events {
		handlerKey := "nguyen_" + handlerName + "_" + evName
		events = append(events, eventReg{key: handlerKey, handler: handlerName})
		if node.attrs == nil {
			node.attrs = make(map[string]string)
		}
		node.attrs["data-nguyen-"+evName] = handlerKey
	}
	for _, child := range node.children {
		events = append(events, collectEvents(child)...)
	}
	return events
}

// parseHTML converts an HTML string into a tree of htmlNodes.
// Uses a simple tokenizing approach suitable for .gox templates.
func parseHTML(html string) *htmlNode {
	tokens := tokenizeHTML(html)
	if len(tokens) == 0 {
		return &htmlNode{isText: true, text: html}
	}

	var root *htmlNode
	var stack []*htmlNode

	for _, tok := range tokens {
		switch tok.kind {
		case tokenOpen:
			node := &htmlNode{
				tag:         tok.tag,
				attrs:       tok.attrs,
				events:      tok.events,
				isSelfClose: tok.selfClose,
			}
			if node.attrs == nil {
				node.attrs = make(map[string]string)
			}
			if node.events == nil {
				node.events = make(map[string]string)
			}

			if len(stack) == 0 {
				root = node
			} else {
				stack[len(stack)-1].addChild(node)
			}

			if !tok.selfClose {
				stack = append(stack, node)
			}

		case tokenClose:
			tagName := tok.tag
			for i := len(stack) - 1; i >= 0; i-- {
				if stack[i].tag == tagName {
					stack = stack[:i]
					break
				}
				stack = stack[:i]
			}

		case tokenText:
			text := strings.TrimSpace(tok.text)
			if text == "" {
				continue
			}
			textNode := &htmlNode{isText: true, text: text}
			if len(stack) == 0 {
				if root == nil {
					root = textNode
				}
			} else {
				stack[len(stack)-1].addChild(textNode)
			}
		}
	}

	// If no root, wrap bare text
	if root == nil {
		return &htmlNode{isText: true, text: html}
	}
	return root
}

type tokenKind int

const (
	tokenOpen  tokenKind = 0
	tokenClose tokenKind = 1
	tokenText  tokenKind = 2
)

type htmlToken struct {
	kind      tokenKind
	tag       string
	attrs     map[string]string
	events    map[string]string
	text      string
	selfClose bool
}

// tokenizeHTML converts an HTML string into a flat list of tokens (open/close/text).
func tokenizeHTML(html string) []htmlToken {
	var tokens []htmlToken
	remaining := html

	for remaining != "" {
		remaining = strings.TrimSpace(remaining)
		if remaining == "" {
			break
		}

		if strings.HasPrefix(remaining, "<") {
			// Check for closing tag
			if strings.HasPrefix(remaining, "</") {
				end := strings.Index(remaining, ">")
				if end == -1 {
					break
				}
				tagName := strings.TrimPrefix(remaining[:end], "</")
				tagName = strings.TrimSpace(tagName)
				tokens = append(tokens, htmlToken{kind: tokenClose, tag: tagName})
				remaining = remaining[end+1:]
				continue
			}

			// Opening tag
			end := strings.Index(remaining, ">")
			if end == -1 {
				break
			}
			tagContent := remaining[1:end] // strip < and >
			selfClose := strings.HasSuffix(tagContent, "/")
			if selfClose {
				tagContent = strings.TrimSuffix(tagContent, "/")
				tagContent = strings.TrimSpace(tagContent)
			}

			parts := strings.SplitN(tagContent, " ", 2)
			tagName := strings.TrimSpace(parts[0])

			attrs := make(map[string]string)
			events := make(map[string]string)

			if len(parts) > 1 {
				attrStr := parts[1]
				matches := attrPattern.FindAllStringSubmatch(attrStr, -1)
				for _, m := range matches {
					key := m[1]
					value := m[2]
					if strings.HasPrefix(key, "@") {
						eventName := strings.TrimPrefix(key, "@")
						handlerName := strings.TrimSuffix(value, "()")
						events[eventName] = handlerName
					} else {
						attrs[key] = value
					}
				}
			}

			tokens = append(tokens, htmlToken{
				kind:      tokenOpen,
				tag:       tagName,
				attrs:     attrs,
				events:    events,
				selfClose: selfClose,
			})
			remaining = remaining[end+1:]

		} else {
			// Text content
			nextTag := strings.Index(remaining, "<")
			if nextTag == -1 {
				tokens = append(tokens, htmlToken{kind: tokenText, text: remaining})
				break
			}
			if nextTag > 0 {
				tokens = append(tokens, htmlToken{kind: tokenText, text: remaining[:nextTag]})
			}
			remaining = remaining[nextTag:]
		}
	}

	return tokens
}

// -------------------------------------------
// VNode Expression Emitters
// -------------------------------------------

// isLiteralTag returns true for tags whose text content should not be interpolated.
func isLiteralTag(tag string) bool {
	return tag == "code" || tag == "pre"
}

// emitVNodeExpr writes a Go expression for a VNode (no "return" keyword)
func emitVNodeExpr(sb *strings.Builder, node *htmlNode, indent string) {
	emitVNodeExprSkip(sb, node, indent, false)
}

func emitVNodeExprSkip(sb *strings.Builder, node *htmlNode, indent string, skipInterp bool) {
	if node.isText {
		if skipInterp {
			sb.WriteString("core.Text(\"" + escapeForGo(node.text) + "\")")
		} else {
			sb.WriteString(formatVNodeText(node.text))
		}
		return
	}

	// Handle control flow directives
	switch node.tag {
	case "ng-if":
		emitNgIfExpr(sb, node, indent)
		return
	case "ng-for":
		emitNgForExpr(sb, node, indent)
		return
	case "ng-else-if", "ng-else":
		sb.WriteString("nil")
		return
	}

	// Handle island directives: elements with data-nguyen-island are compiled to separate WASM
	if node.attrs["data-nguyen-island"] != "" {
		islandID := node.attrs["data-nguyen-island"]
		if node.attrs == nil {
			node.attrs = make(map[string]string)
		}
		node.attrs["data-nguyen-island-id"] = islandID
	}

	// Handle nguyen-* built-in components
	if strings.HasPrefix(node.tag, "nguyen-") {
		emitNguyenVNodeExpr(sb, node, indent)
		return
	}

	childSkip := skipInterp || isLiteralTag(node.tag)
	hasProps := len(node.attrs) > 0
	hasChildren := len(node.children) > 0

	if hasChildren {
		sb.WriteString("core.H(\"" + node.tag + "\", " + formatProps(node.attrs) + ",\n")
		emitChildrenSkip(sb, node.children, indent+"\t", childSkip)
		sb.WriteString(")")
	} else if hasProps {
		sb.WriteString("core.H(\"" + node.tag + "\", " + formatProps(node.attrs) + ")")
	} else {
		sb.WriteString("core.H(\"" + node.tag + "\", nil)")
	}
}

// emitChildren writes child VNode expressions, handling ng-if chains.
func emitChildren(sb *strings.Builder, children []*htmlNode, indent string) {
	emitChildrenSkip(sb, children, indent, false)
}

func emitChildrenSkip(sb *strings.Builder, children []*htmlNode, indent string, skipInterp bool) {
	for i := 0; i < len(children); {
		child := children[i]
		if child.tag == "ng-if" {
			emitIfChain(sb, child, children, &i, indent)
		} else if child.tag == "ng-else-if" || child.tag == "ng-else" {
			i++ // already consumed by emitIfChain
		} else {
			sb.WriteString(indent)
			emitVNodeExprSkip(sb, child, indent, skipInterp)
			if i < len(children)-1 {
				sb.WriteString(",\n")
			}
			i++
		}
	}
}

// emitIfChain emits a Go conditional expression covering ng-if + any adjacent ng-else-if / ng-else siblings.
func emitIfChain(sb *strings.Builder, ifNode *htmlNode, siblings []*htmlNode, idx *int, indent string) {
	cond, ok := isSafeGoExpr(ifNode.attrs["condition"])
	if !ok {
		cond = "false"
	}
	sb.WriteString(indent)
	sb.WriteString("func() *core.VNode {\n")
	sb.WriteString(indent + "\tif " + cond + " {\n")
	sb.WriteString(indent + "\t\treturn ")
	emitIfChildren(sb, ifNode.children, indent+"\t\t")
	sb.WriteString("\n")

	j := *idx + 1
	for j < len(siblings) {
		sib := siblings[j]
		if sib.tag == "ng-else-if" {
			elseCond, ok := isSafeGoExpr(sib.attrs["condition"])
			if !ok {
				elseCond = "false"
			}
			sb.WriteString(indent + "\t} else if " + elseCond + " {\n")
			sb.WriteString(indent + "\t\treturn ")
			emitIfChildren(sb, sib.children, indent+"\t\t")
			sb.WriteString("\n")
			j++
		} else if sib.tag == "ng-else" {
			sb.WriteString(indent + "\t} else {\n")
			sb.WriteString(indent + "\t\treturn ")
			emitIfChildren(sb, sib.children, indent+"\t\t")
			sb.WriteString("\n")
			j++
			break
		} else {
			break
		}
	}
	sb.WriteString(indent + "\t}\n")
	sb.WriteString(indent + "\treturn nil\n")
	sb.WriteString(indent + "}()")
	*idx = j
}

// emitNgIfExpr emits a Go conditional for a single <ng-if ...> block.
// It merges adjacent <ng-if>, <ng-else-if>, <ng-else> into a single Go expression.
func emitNgIfExpr(sb *strings.Builder, node *htmlNode, indent string) {
	// For standalone <ng-if>, emit a simple if/return without else-if chaining here.
	// When called from emitChildren, the chain is handled by emitIfChain.
	// This path is for <ng-if> that is the root of the template.
	cond, ok := isSafeGoExpr(node.attrs["condition"])
	if !ok {
		cond = "false"
	}
	sb.WriteString("func() *core.VNode {\n")
	sb.WriteString(indent + "\tif " + cond + " {\n")
	sb.WriteString(indent + "\t\treturn ")
	emitIfChildren(sb, node.children, indent+"\t\t")
	sb.WriteString("\n")
	sb.WriteString(indent + "\t}\n")
	sb.WriteString(indent + "\treturn nil\n")
	sb.WriteString(indent + "}()")
}

// emitNgForExpr emits a Go expression for <ng-for> that returns a *core.VNode.
// The loop iterates over the slice and wraps children in a specified tag (default div).
// Example: <ng-for items="users" item="user" as="ul">...</ng-for>
func emitNgForExpr(sb *strings.Builder, node *htmlNode, indent string) {
	rawItems := node.attrs["items"]
	itemVar := node.attrs["item"]
	indexVar := node.attrs["index"]
	wrapTag := node.attrs["as"]
	items, ok := isSafeGoExpr(rawItems)
	if !ok || items == "" {
		sb.WriteString("nil")
		return
	}
	if !isSafeIdent(itemVar) {
		itemVar = "item"
	}
	if !isSafeIdent(indexVar) {
		indexVar = "i"
	}
	if !isSafeTagName(wrapTag) {
		wrapTag = "div"
	}

	// The generated Go code must account for the fact that in Go,
	// `range` on a slice produces the same types the slice holds.
	// The user must declare `items` as a typed slice in frontmatter.
	sb.WriteString("func() *core.VNode {\n")
	sb.WriteString(indent + "\tvar _ngKids []interface{}\n")
	sb.WriteString(indent + "\tfor " + indexVar + ", " + itemVar + " := range " + items + " {\n")
	sb.WriteString(indent + "\t\t_ngKids = append(_ngKids, ")
	if len(node.children) == 0 {
		sb.WriteString("nil")
	} else if len(node.children) == 1 {
		emitVNodeExpr(sb, node.children[0], indent+"\t\t")
	} else {
		sb.WriteString("core.H(\"div\", nil,\n")
		for i, child := range node.children {
			sb.WriteString(indent + "\t\t\t")
			emitVNodeExpr(sb, child, indent+"\t\t\t")
			if i < len(node.children)-1 {
				sb.WriteString(",\n")
			}
		}
		sb.WriteString(")")
	}
	sb.WriteString(")\n")
	sb.WriteString(indent + "\t}\n")
	sb.WriteString(indent + "\treturn core.H(\"" + wrapTag + "\", nil, _ngKids...)\n")
	sb.WriteString(indent + "}()")
}

// emitNguyenVNodeExpr writes a Go expression for a nguyen-* component
func emitNguyenVNodeExpr(sb *strings.Builder, node *htmlNode, indent string) {
	switch node.tag {
	case "nguyen-link":
		href := sanitizeHref(node.attrs["href"])
		cls := node.attrs["class"]
		props := "core.Attr{\"href\": \"" + escapeForGo(href) + "\", \"class\": \"" + escapeForGo(cls) + "\", \"data-nguyen-link\": \"\"}"
		if _, ok := node.attrs["prefetch"]; ok {
			props = "core.Attr{\"href\": \"" + escapeForGo(href) + "\", \"class\": \"" + escapeForGo(cls) + "\", \"data-nguyen-link\": \"\", \"data-prefetch\": \"\"}"
		}
		sb.WriteString("core.H(\"a\", " + props)
		for _, child := range node.children {
			sb.WriteString(",\n" + indent + "\t")
			emitVNodeExpr(sb, child, indent+"\t")
		}
		sb.WriteString(")")

	case "nguyen-slot":
		name := node.attrs["name"]
		if name == "" {
			sb.WriteString("core.H(\"div\", core.Attr{\"data-nguyen-slot\": \"\"})")
		} else {
			sb.WriteString("core.H(\"div\", core.Attr{\"data-nguyen-slot\": \"" + escapeForGo(name) + "\"})")
		}

	case "nguyen-head":
		title := node.attrs["title"]
		desc := node.attrs["description"]
		sb.WriteString("func() *core.VNode { core.NguyenHead(\"" + escapeForGo(title) + "\", \"" + escapeForGo(desc) + "\"); return nil }()")

	case "nguyen-image":
		src := node.attrs["src"]
		alt := node.attrs["alt"]
		props := "core.Attr{\"src\": \"" + escapeForGo(src) + "\", \"alt\": \"" + escapeForGo(alt) + "\", \"loading\": \"lazy\", \"decoding\": \"async\", \"data-nguyen-img\": \"" + escapeForGo(src) + "\""
		if w, ok := node.attrs["width"]; ok {
			props += ", \"width\": \"" + escapeForGo(w) + "\""
		}
		if h, ok := node.attrs["height"]; ok {
			props += ", \"height\": \"" + escapeForGo(h) + "\""
		}
		props += "}"
		sb.WriteString("core.H(\"img\", " + props + ")")

	default:
		sb.WriteString("core.H(\"" + node.tag + "\", " + formatProps(node.attrs) + ")")
	}
}

// emitIfChildren emits children as a single return expression for if/else bodies.
func emitIfChildren(sb *strings.Builder, children []*htmlNode, indent string) {
	if len(children) == 0 {
		sb.WriteString("nil")
		return
	}
	if len(children) == 1 {
		emitVNodeExpr(sb, children[0], indent)
		return
	}
	// Multiple children → wrap in a fragment (div placeholder)
	sb.WriteString("core.H(\"div\", nil,\n")
	for i, child := range children {
		sb.WriteString(indent + "\t")
		emitVNodeExpr(sb, child, indent+"\t")
		if i < len(children)-1 {
			sb.WriteString(",\n")
		}
	}
	sb.WriteString(")")
}

// formatVNodeText emits the Go expression for a text node.
// Handles {var} interpolation: "Count: {count}" → `fmt.Sprintf("Count: %v", count)`
func formatVNodeText(text string) string {
	if !varPattern.MatchString(text) {
		return "core.Text(\"" + escapeForGo(text) + "\")"
	}

	parts := varPattern.Split(text, -1)
	matches := varPattern.FindAllStringSubmatch(text, -1)

	var fmtStr strings.Builder
	var args []string
	for i, part := range parts {
		fmtStr.WriteString(part)
		if i < len(matches) {
			fmtStr.WriteString("%v")
			args = append(args, matches[i][1])
		}
	}

	return "core.Text(fmt.Sprintf(\"" + escapeForGo(fmtStr.String()) + "\", " + strings.Join(args, ", ") + "))"
}

// formatProps builds a Go map literal from attrs: `core.Attr{"key": "val", ...}`
func formatProps(attrs map[string]string) string {
	if len(attrs) == 0 {
		return "nil"
	}
	var parts []string
	// Sort keys for deterministic output
	keys := sortedKeys(attrs)
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%q: %q", k, attrs[k]))
	}
	return "core.Attr{" + strings.Join(parts, ", ") + "}"
}

func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	for i := 1; i < len(keys); i++ {
		for j := i; j > 0 && keys[j] < keys[j-1]; j-- {
			keys[j], keys[j-1] = keys[j-1], keys[j]
		}
	}
	return keys
}

// escapeForGo escapes a string for safe inclusion in Go double-quoted string literals
func escapeForGo(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "\"", "\\\"")
	s = strings.ReplaceAll(s, "\n", "\\n")
	s = strings.ReplaceAll(s, "\r", "\\r")
	s = strings.ReplaceAll(s, "\t", "\\t")
	return s
}
