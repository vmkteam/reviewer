package direct

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
	"time"
)

const (
	astIndexBin = "ast-index"
	astClip     = 60_000
	astTimeout  = 30 * time.Second
)

// astArgRe validates symbol/query arguments — letters, digits, _ . * — so a
// value can never be interpreted as a flag (no leading dash, no spaces).
var astArgRe = regexp.MustCompile(`^[A-Za-z_*][\w.*]*$`)

func validAstArg(s string) bool {
	return s != "" && !strings.HasPrefix(s, "-") && astArgRe.MatchString(s)
}

// astIndexAvailable reports whether the ast-index binary is on PATH.
func astIndexAvailable() bool {
	_, err := exec.LookPath(astIndexBin)
	return err == nil
}

// EnsureAstIndex does a full rebuild of the ast-index for root, so the index
// always reflects the current working tree (including uncommitted and untracked
// files under review). No-op and nil when the binary is absent. Best-effort —
// callers log and continue on error. Returns whether a rebuild ran.
func EnsureAstIndex(ctx context.Context, root string) (bool, error) {
	if !astIndexAvailable() {
		return false, nil
	}
	c, cancel := context.WithTimeout(ctx, 4*astTimeout)
	defer cancel()
	cmd := exec.CommandContext(c, astIndexBin, "rebuild")
	cmd.Dir = root
	if out, err := cmd.CombinedOutput(); err != nil {
		return true, fmt.Errorf("ast-index rebuild: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return true, nil
}

// registerAstTools adds the ast-index-backed navigation tools. Only called when
// the binary is available.
func registerAstTools(reg *Registry, root, defBase string) {
	reg.Register(astSymbolTool(root))
	reg.Register(astSearchTool(root))
	reg.Register(astRefsTool(root))
	reg.Register(astCallersTool(root))
	reg.Register(astOutlineTool(root))
	reg.Register(astChangedTool(root, defBase))
}

func runAstIndex(ctx context.Context, root string, args ...string) (string, error) {
	c, cancel := context.WithTimeout(ctx, astTimeout)
	defer cancel()
	cmd := exec.CommandContext(c, astIndexBin, args...)
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) && len(ee.Stderr) > 0 {
			return "", fmt.Errorf("%s", strings.TrimSpace(string(ee.Stderr)))
		}
		return "", err
	}
	res := strings.TrimSpace(string(out))
	if res == "" {
		return "(no results)", nil
	}
	return clipN(res, astClip), nil
}

// astNameTool builds a tool that passes a single validated symbol-like argument
// to `ast-index <cmd> <name>`.
func astNameTool(root, name, cmd, arg, desc string) (ToolDef, Handler) {
	def := ToolDef{
		Name:        name,
		Description: desc,
		Schema:      objSchema(map[string]any{arg: strProp("Symbol or function name")}, arg),
	}
	h := func(ctx context.Context, raw json.RawMessage) (string, error) {
		var a map[string]string
		if err := json.Unmarshal(raw, &a); err != nil {
			return "", fmt.Errorf("%s: bad arguments: %w", name, err)
		}
		v := a[arg]
		if !validAstArg(v) {
			return "", fmt.Errorf("%s: invalid %s %q", name, arg, v)
		}
		return runAstIndex(ctx, root, cmd, v)
	}
	return def, h
}

func astSymbolTool(root string) (ToolDef, Handler) {
	return astNameTool(root, "ast_symbol", "symbol", "name",
		"Find a symbol's definition and signature (function/type/etc) via the AST index. Precise; prefer over grep for navigation.")
}

func astSearchTool(root string) (ToolDef, Handler) {
	return astNameTool(root, "ast_search", "search", "query",
		"Search the AST index for files and symbols matching a name.")
}

func astRefsTool(root string) (ToolDef, Handler) {
	return astNameTool(root, "ast_refs", "refs", "symbol",
		"Show cross-references (definitions, imports, usages) of a symbol — for impact analysis.")
}

func astCallersTool(root string) (ToolDef, Handler) {
	return astNameTool(root, "ast_callers", "callers", "function",
		"Find callers of a function — who would be affected by a change.")
}

func astOutlineTool(root string) (ToolDef, Handler) {
	def := ToolDef{
		Name:        "ast_outline",
		Description: "List the symbols (functions/types) declared in a file via the AST index.",
		Schema:      objSchema(map[string]any{fFile: strProp("Repository-relative file path")}, fFile),
	}
	h := func(ctx context.Context, raw json.RawMessage) (string, error) {
		var a struct {
			File string `json:"file"`
		}
		if err := json.Unmarshal(raw, &a); err != nil {
			return "", fmt.Errorf("ast_outline: bad arguments: %w", err)
		}
		if _, err := resolveInRoot(root, a.File); err != nil {
			return "", fmt.Errorf("ast_outline: %w", err)
		}
		return runAstIndex(ctx, root, "outline", a.File)
	}
	return def, h
}

func astChangedTool(root, defBase string) (ToolDef, Handler) {
	def := ToolDef{
		Name:        "ast_changed",
		Description: "List symbols changed vs the base branch (committed changes only). Good for focusing the review on what moved.",
		Schema:      objSchema(map[string]any{"base": strProp("Base ref (optional; defaults to the target branch)")}),
	}
	h := func(ctx context.Context, raw json.RawMessage) (string, error) {
		var a struct {
			Base string `json:"base"`
		}
		if len(raw) > 0 {
			if err := json.Unmarshal(raw, &a); err != nil {
				return "", fmt.Errorf("ast_changed: bad arguments: %w", err)
			}
		}
		base := a.Base
		if base == "" {
			base = defBase
		}
		if base != "" && !validRef(base) {
			return "", fmt.Errorf("ast_changed: invalid base %q", base)
		}
		args := []string{"changed"}
		if base != "" {
			args = append(args, "--base", base)
		}
		return runAstIndex(ctx, root, args...)
	}
	return def, h
}
