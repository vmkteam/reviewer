package direct

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
)

const (
	maxGlobResults = 300
	maxGrepMatches = 300
	maxGrepFile    = 1 << 20 // skip files larger than 1 MiB in grep
)

// Tool names and schema property keys reused across the filesystem tools.
const (
	toolReadFile = "read_file"
	toolGlob     = "glob"
	toolGrep     = "grep"
	fPath        = "path"
)

// skipDirs are never descended into by glob/grep walks.
var skipDirs = map[string]bool{".git": true, "node_modules": true, "vendor": true, "dist": true}

// readTracker records which files have already been fully read in this session
// (seeded with the pre-loaded changed files), so a repeat full read returns a
// short stub instead of re-sending the content. A review is read-only, so a
// file's content does not change between reads. Safe for concurrent use —
// read tools run in parallel.
type readTracker struct {
	mu   sync.Mutex
	seen map[string]bool
}

func newReadTracker(seed []string) *readTracker {
	t := &readTracker{seen: make(map[string]bool, len(seed))}
	for _, p := range seed {
		t.seen[normPath(p)] = true
	}
	return t
}

// firstRead marks path as fully read and reports whether this is the first time.
// A false return means the caller should emit a dedup stub.
func (t *readTracker) firstRead(path string) bool {
	key := normPath(path)
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.seen[key] {
		return false
	}
	t.seen[key] = true
	return true
}

func normPath(p string) string { return filepath.ToSlash(filepath.Clean(p)) }

func readDedupStub(path string) string {
	return "[already provided: " + path + " — its full content is already in the conversation " +
		"(pre-loaded or read earlier). Reason from it; do not re-read. Pass offset/limit to read_file " +
		"if you need a specific line range.]"
}

// resolveInRoot joins p onto root and verifies the result stays inside root,
// rejecting absolute paths and traversal that escapes the tree.
func resolveInRoot(root, p string) (string, error) {
	if strings.TrimSpace(p) == "" {
		return "", errors.New("path is required")
	}
	clean := filepath.Clean(p)
	if filepath.IsAbs(clean) {
		return "", fmt.Errorf("absolute paths are not allowed: %q", p)
	}
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	abs, err := filepath.Abs(filepath.Join(rootAbs, clean))
	if err != nil {
		return "", err
	}
	sep := string(filepath.Separator)
	if abs != rootAbs && !strings.HasPrefix(abs, rootAbs+sep) {
		return "", fmt.Errorf("path escapes repository root: %q", p)
	}
	// The lexical check above is fooled by a symlink inside the tree that points
	// outside it (e.g. evil.txt -> /etc/passwd): the path stays in-root but the
	// read follows the link out. Resolve symlinks and re-verify. EvalSymlinks
	// fails for a not-yet-existing path — that's fine here, such a path can't be
	// read anyway, so let the caller's file op surface the real error.
	if resolved, rerr := filepath.EvalSymlinks(abs); rerr == nil {
		rootResolved, rrerr := filepath.EvalSymlinks(rootAbs)
		if rrerr != nil {
			rootResolved = rootAbs
		}
		if resolved != rootResolved && !strings.HasPrefix(resolved, rootResolved+sep) {
			return "", fmt.Errorf("path escapes repository root via symlink: %q", p)
		}
	}
	return abs, nil
}

// readFileTool reads a file, optionally a 1-based line range.
func readFileTool(root string, rt *readTracker) (ToolDef, Handler) {
	def := ToolDef{
		Name:        toolReadFile,
		Description: "Read a file from the repository. Optionally pass a 1-based line offset and a line limit to read a slice.",
		Schema: objSchema(map[string]any{
			fPath:    strProp("File path relative to the repository root"),
			"offset": intProp("1-based start line (optional)"),
			"limit":  intProp("Maximum number of lines to read (optional)"),
		}, fPath),
	}
	h := func(_ context.Context, raw json.RawMessage) (string, error) {
		var a struct {
			Path   string `json:"path"`
			Offset int    `json:"offset"`
			Limit  int    `json:"limit"`
		}
		if err := json.Unmarshal(raw, &a); err != nil {
			return "", fmt.Errorf("read_file: bad arguments: %w", err)
		}
		abs, err := resolveInRoot(root, a.Path)
		if err != nil {
			return "", fmt.Errorf("read_file: %w", err)
		}
		// Dedup full reads: a file already provided (pre-load or earlier read)
		// returns a stub instead of re-sending its content. Ranged reads
		// (offset/limit) are always served — they fetch a specific slice.
		fullRead := a.Offset <= 0 && a.Limit <= 0
		if fullRead && !rt.firstRead(a.Path) {
			return readDedupStub(a.Path), nil
		}
		data, err := os.ReadFile(abs)
		if err != nil {
			return "", fmt.Errorf("read_file: %w", err)
		}
		if fullRead {
			return clip(string(data)), nil
		}
		lines := strings.Split(string(data), "\n")
		start := 0
		if a.Offset > 0 {
			start = a.Offset - 1
		}
		if start > len(lines) {
			start = len(lines)
		}
		end := len(lines)
		if a.Limit > 0 && start+a.Limit < end {
			end = start + a.Limit
		}
		return clip(strings.Join(lines[start:end], "\n")), nil
	}
	return def, h
}

const (
	maxReadFilesCount = 40
	perFileClip       = 50_000
)

// readFilesTool reads several files in one call, so the model can fan out reads
// in a single step instead of one read_file per turn.
func readFilesTool(root string, rt *readTracker) (ToolDef, Handler) {
	def := ToolDef{
		Name: "read_files",
		Description: "Read several files in ONE call (prefer this over many read_file calls). " +
			"Returns each file's full content under a '===== path =====' header.",
		Schema: objSchema(map[string]any{
			"paths": arrayOfDesc(primProp(jsString, ""), "Repository-relative file paths to read"),
		}, "paths"),
	}
	h := func(_ context.Context, raw json.RawMessage) (string, error) {
		var a struct {
			Paths []string `json:"paths"`
		}
		if err := json.Unmarshal(raw, &a); err != nil {
			return "", fmt.Errorf("read_files: bad arguments: %w", err)
		}
		if len(a.Paths) == 0 {
			return "", errors.New("read_files: paths is empty")
		}
		var b strings.Builder
		for i, p := range a.Paths {
			if i >= maxReadFilesCount {
				fmt.Fprintf(&b, "\n... [%d more paths omitted; call read_files again]\n", len(a.Paths)-i)
				break
			}
			fmt.Fprintf(&b, "===== %s =====\n", p)
			if !rt.firstRead(p) {
				b.WriteString(readDedupStub(p))
				b.WriteString("\n\n")
				continue
			}
			abs, err := resolveInRoot(root, p)
			if err != nil {
				fmt.Fprintf(&b, "ERROR: %s\n\n", err)
				continue
			}
			data, err := os.ReadFile(abs)
			if err != nil {
				fmt.Fprintf(&b, "ERROR: %s\n\n", err)
				continue
			}
			b.WriteString(clipN(string(data), perFileClip))
			b.WriteString("\n\n")
		}
		return clipN(b.String(), 3*defaultClip), nil
	}
	return def, h
}

// globTool lists files matching a glob pattern (supports ** for any depth).
func globTool(root string) (ToolDef, Handler) {
	def := ToolDef{
		Name:        toolGlob,
		Description: "Find files by glob pattern (use ** to match any depth, e.g. **/*.go). Returns paths relative to the repository root.",
		Schema: objSchema(map[string]any{
			"pattern": strProp("Glob pattern, e.g. **/*.go or pkg/**/*_test.go"),
			fPath:     strProp("Subdirectory to search within (optional)"),
		}, "pattern"),
	}
	h := func(_ context.Context, raw json.RawMessage) (string, error) {
		var a struct {
			Pattern string `json:"pattern"`
			Path    string `json:"path"`
		}
		if err := json.Unmarshal(raw, &a); err != nil {
			return "", fmt.Errorf("glob: bad arguments: %w", err)
		}
		re, err := globToRegexp(a.Pattern)
		if err != nil {
			return "", fmt.Errorf("glob: invalid pattern %q: %w", a.Pattern, err)
		}
		rootAbs, err := filepath.Abs(root)
		if err != nil {
			return "", err
		}
		base := rootAbs
		if a.Path != "" {
			if base, err = resolveInRoot(root, a.Path); err != nil {
				return "", fmt.Errorf("glob: %w", err)
			}
		}
		var matches []string
		_ = filepath.WalkDir(base, globWalk(rootAbs, re, &matches))
		sort.Strings(matches)
		truncated := false
		if len(matches) > maxGlobResults {
			matches = matches[:maxGlobResults]
			truncated = true
		}
		if len(matches) == 0 {
			return "no files matched", nil
		}
		out := strings.Join(matches, "\n")
		if truncated {
			out += fmt.Sprintf("\n... [truncated to %d results]", maxGlobResults)
		}
		return out, nil
	}
	return def, h
}

// globWalk returns a WalkDir callback that collects rootAbs-relative paths
// matching re into matches, skipping the configured directories.
func globWalk(rootAbs string, re *regexp.Regexp, matches *[]string) fs.WalkDirFunc {
	return func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil //nolint:nilerr // skip unreadable entries during best-effort walk
		}
		if d.IsDir() {
			if skipDirs[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		rel, rerr := filepath.Rel(rootAbs, p)
		if rerr != nil {
			return nil //nolint:nilerr // skip entries whose relative path can't be computed
		}
		rel = filepath.ToSlash(rel)
		if re.MatchString(rel) {
			*matches = append(*matches, rel)
		}
		return nil
	}
}

// grepTool searches file contents with a Go regexp, optionally filtered by glob.
func grepTool(root string) (ToolDef, Handler) {
	def := ToolDef{
		Name:        toolGrep,
		Description: "Search file contents with a regular expression. Returns matching lines as path:line:text. Optionally restrict to a subdirectory and/or a glob.",
		Schema: objSchema(map[string]any{
			"pattern": strProp("Go regular expression"),
			fPath:     strProp("Subdirectory to search within (optional)"),
			toolGlob:  strProp("Only search files matching this glob (optional)"),
		}, "pattern"),
	}
	h := func(_ context.Context, raw json.RawMessage) (string, error) {
		var a struct {
			Pattern string `json:"pattern"`
			Path    string `json:"path"`
			Glob    string `json:"glob"`
		}
		if err := json.Unmarshal(raw, &a); err != nil {
			return "", fmt.Errorf("grep: bad arguments: %w", err)
		}
		re, err := regexp.Compile(a.Pattern)
		if err != nil {
			return "", fmt.Errorf("grep: invalid pattern: %w", err)
		}
		var globRe *regexp.Regexp
		if a.Glob != "" {
			if globRe, err = globToRegexp(a.Glob); err != nil {
				return "", fmt.Errorf("grep: invalid glob: %w", err)
			}
		}
		rootAbs, err := filepath.Abs(root)
		if err != nil {
			return "", err
		}
		base := rootAbs
		if a.Path != "" {
			if base, err = resolveInRoot(root, a.Path); err != nil {
				return "", fmt.Errorf("grep: %w", err)
			}
		}

		var out []string
		walkErr := filepath.WalkDir(base, grepWalk(rootAbs, re, globRe, &out))
		if len(out) == 0 {
			return "no matches", nil
		}
		res := strings.Join(out, "\n")
		if errors.Is(walkErr, errGrepLimit) {
			res += fmt.Sprintf("\n... [truncated to %d matches]", maxGrepMatches)
		}
		return clip(res), nil
	}
	return def, h
}

// errGrepLimit aborts the grep walk once maxGrepMatches lines are collected.
var errGrepLimit = errors.New("grep limit reached")

// grepWalk returns a WalkDir callback that appends "rel:line:text" matches of re
// to out, honouring the optional glob filter and the match cap.
func grepWalk(rootAbs string, re, globRe *regexp.Regexp, out *[]string) fs.WalkDirFunc {
	return func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil //nolint:nilerr // skip unreadable entries during best-effort walk
		}
		if d.IsDir() {
			if skipDirs[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		rel, rerr := filepath.Rel(rootAbs, p)
		if rerr != nil {
			return nil //nolint:nilerr // skip entries whose relative path can't be computed
		}
		rel = filepath.ToSlash(rel)
		if globRe != nil && !globRe.MatchString(rel) {
			return nil
		}
		if info, ierr := d.Info(); ierr == nil && info.Size() > maxGrepFile {
			return nil
		}
		data, rerr := os.ReadFile(p)
		if rerr != nil || bytes.IndexByte(data, 0) >= 0 {
			return nil //nolint:nilerr // skip unreadable or binary files
		}
		return grepLines(rel, data, re, out)
	}
}

// grepLines appends every line of data matching re to out, returning errGrepLimit
// once the match cap is reached.
func grepLines(rel string, data []byte, re *regexp.Regexp, out *[]string) error {
	for i, line := range strings.Split(string(data), "\n") {
		if re.MatchString(line) {
			*out = append(*out, fmt.Sprintf("%s:%d:%s", rel, i+1, strings.TrimSpace(line)))
			if len(*out) >= maxGrepMatches {
				return errGrepLimit
			}
		}
	}
	return nil
}

// globToRegexp converts a glob (with ** for any-depth) to an anchored regexp
// matching a slash-separated relative path. * matches within a path segment,
// ** matches across segments.
func globToRegexp(pattern string) (*regexp.Regexp, error) {
	var b strings.Builder
	b.WriteString("^")
	for i := 0; i < len(pattern); i++ {
		c := pattern[i]
		switch c {
		case '*':
			if i+1 < len(pattern) && pattern[i+1] == '*' {
				i++ // consume second '*'
				if i+1 < len(pattern) && pattern[i+1] == '/' {
					i++                   // consume the slash too
					b.WriteString(".*/?") // **/ matches zero or more directories
				} else {
					b.WriteString(".*")
				}
			} else {
				b.WriteString("[^/]*")
			}
		case '?':
			b.WriteString("[^/]")
		case '.', '+', '(', ')', '|', '^', '$', '{', '}', '[', ']', '\\':
			b.WriteByte('\\')
			b.WriteByte(c)
		default:
			b.WriteByte(c)
		}
	}
	b.WriteString("$")
	return regexp.Compile(b.String())
}
