// Package vet provides static analysis for .gox files in a Nguyen.go
// project. It is the seed of the future Effect Wall: today it walks
// every .gox frontmatter as Go source and reports issues that we know
// will bite at runtime (missing context propagation, blocking sleeps,
// goroutines that ignore cancellation, hard-coded secrets).
//
// vet is intentionally non-fatal — issues are returned as a slice so
// the CLI can print, count and exit with a single status code. New
// rules should add entries to defaultRules and write a focused
// detector function.
package vet

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// Severity classifies a finding. Errors fail vet; Warnings do not.
type Severity int

const (
	SeverityWarn Severity = iota
	SeverityError
)

func (s Severity) String() string {
	if s == SeverityError {
		return "error"
	}
	return "warning"
}

// Issue is a single finding emitted by a rule.
type Issue struct {
	File     string
	Line     int
	Column   int
	Rule     string
	Message  string
	Severity Severity
}

// Rule is a named detector.
type Rule struct {
	Name        string
	Description string
	Run         func(file *ParsedGox) []Issue
}

// ParsedGox holds the lexed frontmatter + template ready for AST walks.
type ParsedGox struct {
	Path       string
	Source     string
	Frontmatter string
	Template   string
	FileSet    *token.FileSet
	AST        *ast.File   // nil if frontmatter is empty/unparseable
	GoLineBase int          // line number of the start of frontmatter inside Path
}

// DefaultRules returns the built-in rule set.
func DefaultRules() []Rule {
	return []Rule{
		ruleContextFirstParam(),
		ruleNoTimeSleepInHandlers(),
		ruleGoroutineCancellation(),
		ruleNoHardcodedSecrets(),
		ruleNoFmtPrintInHandlers(),
		ruleEventHandlerExists(),
	}
}

// Vet walks every .gox file under root and runs rules on each.
func Vet(root string, rules []Rule) ([]Issue, error) {
	if rules == nil {
		rules = DefaultRules()
	}
	var issues []Issue
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			if name := info.Name(); name == "node_modules" || name == ".git" || name == ".nguyen" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(info.Name(), ".gox") {
			return nil
		}
		parsed, perr := parseGox(path)
		if perr != nil {
			issues = append(issues, Issue{
				File: path, Line: 1, Rule: "parse",
				Message: perr.Error(), Severity: SeverityError,
			})
			return nil
		}
		for _, r := range rules {
			issues = append(issues, r.Run(parsed)...)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return issues, nil
}

// parseGox reads a .gox file, splits it on the --- delimiter and parses
// the frontmatter as a Go file wrapped in a synthetic package so go/ast
// can walk it. Lines reported by the AST refer to the synthetic file;
// GoLineBase corrects them for diagnostics that need the raw .gox line.
func parseGox(path string) (*ParsedGox, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	src := string(raw)
	parts := splitFrontmatter(src)
	pg := &ParsedGox{
		Path:    path,
		Source:  src,
		FileSet: token.NewFileSet(),
	}
	if parts[0] == "" && parts[1] == "" {
		// no frontmatter → treat whole file as template
		pg.Template = src
		return pg, nil
	}
	pg.Frontmatter = parts[0]
	pg.Template = parts[1]
	pg.GoLineBase = countLines(src[:strings.Index(src, parts[0])])

	wrapped := "package gox\n\n" + sanitizeFrontmatter(pg.Frontmatter)
	f, err := parser.ParseFile(pg.FileSet, path, wrapped, parser.ParseComments)
	if err != nil {
		// Don't fail vet — parser errors become rule findings instead.
		return pg, nil
	}
	pg.AST = f
	return pg, nil
}

// splitFrontmatter returns [frontmatter, template] or nil if no `---`
// pair exists. Both Unix and Windows line endings are tolerated.
func splitFrontmatter(src string) [2]string {
	const delim = "---"
	idx := strings.Index(src, delim)
	if idx < 0 {
		return [2]string{}
	}
	rest := src[idx+len(delim):]
	end := strings.Index(rest, delim)
	if end < 0 {
		return [2]string{}
	}
	fm := rest[:end]
	tmpl := rest[end+len(delim):]
	fm = strings.TrimLeft(fm, "\n\r")
	tmpl = strings.TrimLeft(tmpl, "\n\r")
	return [2]string{fm, tmpl}
}

// sanitizeFrontmatter strips .gox-only constructs that the Go parser
// chokes on (top-level := short declarations, `export const ...` from
// JS-style metadata, top-level statement expressions). Anything that
// does not parse is dropped from the synthetic file so AST walks still
// see the surviving structure.
func sanitizeFrontmatter(fm string) string {
	lines := strings.Split(fm, "\n")
	out := make([]string, 0, len(lines))
	depth := 0
	for _, line := range lines {
		trim := strings.TrimSpace(line)
		// track block depth so we don't break function bodies
		depth += strings.Count(line, "{") - strings.Count(line, "}")
		if depth > 0 {
			out = append(out, line)
			continue
		}
		if strings.HasPrefix(trim, "export ") {
			out = append(out, "// "+line)
			continue
		}
		// top-level short decl `x := ...` is illegal at file scope.
		if topLevelShortDecl.MatchString(trim) {
			out = append(out, "var _ = "+strings.SplitN(trim, ":=", 2)[1])
			continue
		}
		// bare statements at file scope → comment them
		if isBareStatement(trim) {
			out = append(out, "// "+line)
			continue
		}
		out = append(out, line)
	}
	return strings.Join(out, "\n")
}

var topLevelShortDecl = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*(\s*,\s*[A-Za-z_][A-Za-z0-9_]*)*\s*:=`)

func isBareStatement(line string) bool {
	if line == "" || strings.HasPrefix(line, "//") {
		return false
	}
	if strings.HasPrefix(line, "package ") || strings.HasPrefix(line, "import") {
		return false
	}
	if strings.HasPrefix(line, "func ") || strings.HasPrefix(line, "type ") ||
		strings.HasPrefix(line, "const ") || strings.HasPrefix(line, "var ") {
		return false
	}
	if strings.HasPrefix(line, ")") || strings.HasPrefix(line, "}") {
		return false
	}
	return strings.ContainsAny(line, "(=")
}

func countLines(s string) int {
	return strings.Count(s, "\n") + 1
}

func (p *ParsedGox) lineForPos(pos token.Pos) int {
	if p.FileSet == nil {
		return 0
	}
	pp := p.FileSet.Position(pos)
	if pp.Line == 0 {
		return 0
	}
	// AST line is 1-based against synthetic file (which prepends
	// "package gox\n\n" — 2 extra lines). Adjust back to .gox line.
	return pp.Line - 2 + (p.GoLineBase - 1)
}

// ---------------------------------------------------------------------
// Rules
// ---------------------------------------------------------------------

func ruleContextFirstParam() Rule {
	return Rule{
		Name:        "context-first-param",
		Description: "exported functions taking a request scope should accept context.Context as the first parameter",
		Run: func(p *ParsedGox) []Issue {
			if p.AST == nil {
				return nil
			}
			var out []Issue
			for _, decl := range p.AST.Decls {
				fn, ok := decl.(*ast.FuncDecl)
				if !ok || fn.Recv != nil {
					continue
				}
				name := fn.Name.Name
				if !isHandlerName(name) {
					continue
				}
				if fn.Type.Params == nil || len(fn.Type.Params.List) == 0 {
					continue
				}
				first := fn.Type.Params.List[0]
				if !isContextType(first.Type) {
					out = append(out, Issue{
						File: p.Path, Line: p.lineForPos(fn.Pos()),
						Rule: "context-first-param", Severity: SeverityWarn,
						Message: fmt.Sprintf("function %q is a request handler but its first parameter is not context.Context", name),
					})
				}
			}
			return out
		},
	}
}

func isHandlerName(name string) bool {
	switch name {
	case "Load", "Action", "Stream", "Handle", "GET", "POST", "PUT", "DELETE", "PATCH":
		return true
	}
	return false
}

func isContextType(expr ast.Expr) bool {
	sel, ok := expr.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	id, ok := sel.X.(*ast.Ident)
	if !ok {
		return false
	}
	return id.Name == "context" && sel.Sel.Name == "Context"
}

func ruleNoTimeSleepInHandlers() Rule {
	return Rule{
		Name:        "no-time-sleep",
		Description: "time.Sleep blocks the goroutine without listening for cancellation; use ctx-aware select instead",
		Run: func(p *ParsedGox) []Issue {
			if p.AST == nil {
				return nil
			}
			var out []Issue
			ast.Inspect(p.AST, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}
				if isCallTo(call.Fun, "time", "Sleep") {
					out = append(out, Issue{
						File: p.Path, Line: p.lineForPos(call.Pos()),
						Rule: "no-time-sleep", Severity: SeverityWarn,
						Message: "time.Sleep ignores ctx cancellation — use select { case <-time.After: case <-ctx.Done(): } instead",
					})
				}
				return true
			})
			return out
		},
	}
}

func isCallTo(e ast.Expr, pkg, name string) bool {
	sel, ok := e.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	id, ok := sel.X.(*ast.Ident)
	if !ok {
		return false
	}
	return id.Name == pkg && sel.Sel.Name == name
}

func ruleGoroutineCancellation() Rule {
	return Rule{
		Name:        "goroutine-cancellation",
		Description: "goroutines spawned in handlers should reference ctx.Done() so they exit when the request ends",
		Run: func(p *ParsedGox) []Issue {
			if p.AST == nil {
				return nil
			}
			var out []Issue
			ast.Inspect(p.AST, func(n ast.Node) bool {
				goStmt, ok := n.(*ast.GoStmt)
				if !ok {
					return true
				}
				body := goStmtBody(goStmt)
				if body == "" {
					return true
				}
				if !strings.Contains(body, "ctx") && !strings.Contains(body, "Context") {
					out = append(out, Issue{
						File: p.Path, Line: p.lineForPos(goStmt.Pos()),
						Rule: "goroutine-cancellation", Severity: SeverityWarn,
						Message: "goroutine does not appear to listen on a context — it may outlive its request",
					})
				}
				return true
			})
			return out
		},
	}
}

func goStmtBody(g *ast.GoStmt) string {
	if lit, ok := g.Call.Fun.(*ast.FuncLit); ok && lit.Body != nil {
		var sb strings.Builder
		for _, stmt := range lit.Body.List {
			sb.WriteString(fmt.Sprintf("%T ", stmt))
			ast.Inspect(stmt, func(n ast.Node) bool {
				if id, ok := n.(*ast.Ident); ok {
					sb.WriteString(id.Name + " ")
				}
				return true
			})
		}
		return sb.String()
	}
	return ""
}

var secretPatterns = []*regexp.Regexp{
	// API keys, secret keys, passwords, tokens — at least 8 chars
	regexp.MustCompile(`(?i)(api[_-]?key|secret[_-]?key|password|passwd|token|secret)\s*[:=]\s*"[A-Za-z0-9_\-!@#$%^&*]{8,}"`),
	// AWS/GCP/Azure credentials
	regexp.MustCompile(`(?i)(aws|gcp|azure)[_-]?(secret|key|token|password)\s*[:=]\s*"[A-Za-z0-9_\-/+]{8,}"`),
	// Common service key patterns (Stripe, Twilio, SendGrid, etc.)
	regexp.MustCompile(`(?i)(sk_live_|sk_test_|rk_live_|AC[a-z0-9]{32}|SG\.|AKIA)[A-Za-z0-9_\-]{8,}`),
	// Private key headers
	regexp.MustCompile(`-----BEGIN (RSA |EC |OPENSSH )?PRIVATE KEY-----`),
	// Empty or short secrets assigned to known key names
	regexp.MustCompile(`(?i)(jwt[_-]?secret|auth[_-]?secret|signing[_-]?key)\s*[:=]\s*"[^"]{0,31}"`),
}

func ruleNoHardcodedSecrets() Rule {
	return Rule{
		Name:        "no-hardcoded-secrets",
		Description: "string literals that look like API keys or passwords should be loaded from env vars",
		Run: func(p *ParsedGox) []Issue {
			var out []Issue
			lines := strings.Split(p.Source, "\n")
			for i, line := range lines {
				for _, rx := range secretPatterns {
					if rx.MatchString(line) {
						out = append(out, Issue{
							File: p.Path, Line: i + 1,
							Rule: "no-hardcoded-secrets", Severity: SeverityError,
							Message: "hardcoded secret detected — load from os.Getenv or a secrets manager",
						})
						break
					}
				}
			}
			return out
		},
	}
}

func ruleNoFmtPrintInHandlers() Rule {
	return Rule{
		Name:        "no-fmt-print",
		Description: "fmt.Println / fmt.Printf bypass the structured logger; use observe.Logger(ctx) instead",
		Run: func(p *ParsedGox) []Issue {
			if p.AST == nil {
				return nil
			}
			var out []Issue
			ast.Inspect(p.AST, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}
				sel, ok := call.Fun.(*ast.SelectorExpr)
				if !ok {
					return true
				}
				id, ok := sel.X.(*ast.Ident)
				if !ok || id.Name != "fmt" {
					return true
				}
				if !strings.HasPrefix(sel.Sel.Name, "Print") {
					return true
				}
				out = append(out, Issue{
					File: p.Path, Line: p.lineForPos(call.Pos()),
					Rule: "no-fmt-print", Severity: SeverityWarn,
					Message: "fmt." + sel.Sel.Name + " is unstructured; prefer observe.Logger(ctx).Info(...)",
				})
				return true
			})
			return out
		},
	}
}

var eventBindRx = regexp.MustCompile(`@(\w+)="(\w+)\(\)"`)

func ruleEventHandlerExists() Rule {
	return Rule{
		Name:        "event-handler-exists",
		Description: "every @event=\"name()\" binding must have a matching Go function in frontmatter",
		Run: func(p *ParsedGox) []Issue {
			if p.AST == nil || p.Template == "" {
				return nil
			}
			defined := map[string]bool{}
			for _, decl := range p.AST.Decls {
				fn, ok := decl.(*ast.FuncDecl)
				if !ok || fn.Recv != nil {
					continue
				}
				defined[fn.Name.Name] = true
			}
			var out []Issue
			matches := eventBindRx.FindAllStringSubmatch(p.Template, -1)
			seen := map[string]bool{}
			for _, m := range matches {
				name := m[2]
				if seen[name] {
					continue
				}
				seen[name] = true
				if !defined[name] {
					out = append(out, Issue{
						File: p.Path, Line: 1,
						Rule: "event-handler-exists", Severity: SeverityError,
						Message: fmt.Sprintf("event handler %q is bound in template but not defined in frontmatter", name),
					})
				}
			}
			return out
		},
	}
}
