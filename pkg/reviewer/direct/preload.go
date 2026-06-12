package direct

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	preloadMaxBytes = 250_000
	preloadPerFile  = 60_000
	preloadMaxFiles = 50
)

// PreloadContext returns a kickoff block with the diff under review and the full
// current content of every changed file, so the model can review without reading
// them via tools (the main source of one-call-per-turn round-trips). It also
// returns the list of files actually shown, so read-dedup can be seeded with
// them. Best-effort: returns "", nil if git is unavailable or nothing changed.
func PreloadContext(ctx context.Context, root, base, head string) (string, []string) {
	diff, derr := gitDiff(ctx, root, base, head)
	files, ferr := changedFiles(ctx, root, base, head)
	if (derr != nil || strings.TrimSpace(diff) == "" || diff == emptyDiff) && (ferr != nil || len(files) == 0) {
		return "", nil
	}

	var b strings.Builder
	b.WriteString("## Дифф под ревью и полное содержимое изменённых файлов\n")
	b.WriteString("Ниже — дифф и текущее ПОЛНОЕ содержимое всех изменённых файлов. Ревьюй по ним; ")
	b.WriteString("не вызывай git_diff и read_file для уже показанных здесь файлов.\n\n")

	if strings.TrimSpace(diff) != "" && diff != emptyDiff {
		b.WriteString("### Diff\n```diff\n")
		b.WriteString(clipN(diff, preloadMaxBytes/2))
		b.WriteString("\n```\n\n")
	}

	b.WriteString("### Изменённые файлы (полное содержимое)\n")
	var preloaded []string
	for _, f := range files {
		if len(preloaded) >= preloadMaxFiles || b.Len() > preloadMaxBytes {
			fmt.Fprintf(&b, "\n... [ещё %d файлов не показаны — используй read_files]\n", len(files)-len(preloaded))
			break
		}
		abs := filepath.Join(root, filepath.FromSlash(f))
		data, err := os.ReadFile(abs)
		if err != nil {
			continue // deleted / binary / unreadable — skip silently
		}
		fmt.Fprintf(&b, "===== %s =====\n", f)
		b.WriteString(clipN(string(data), preloadPerFile))
		b.WriteString("\n\n")
		preloaded = append(preloaded, f)
	}
	return b.String(), preloaded
}

// changedFiles lists files changed vs base: the committed range base...head when
// both are set, otherwise the working-tree diff vs base plus untracked files.
func changedFiles(ctx context.Context, root, base, head string) ([]string, error) {
	if base != "" && head != "" {
		out, err := runGit(ctx, root, false, withExcludes("--no-pager", "diff", "--name-only", base+"..."+head)...)
		if err != nil {
			return nil, err
		}
		return dedupeStrings(splitLines(out)), nil
	}

	args := []string{"--no-pager", "diff", "--name-only"}
	if base != "" {
		args = append(args, base)
	}
	out, err := runGit(ctx, root, false, withExcludes(args...)...)
	if err != nil {
		return nil, err
	}
	names := splitLines(out)
	if ut, uerr := runGit(ctx, root, false, withExcludes("ls-files", "--others", "--exclude-standard")...); uerr == nil {
		names = append(names, splitLines(ut)...)
	}
	return dedupeStrings(names), nil
}

func splitLines(s string) []string {
	var out []string
	for _, l := range strings.Split(strings.TrimSpace(s), "\n") {
		if l = strings.TrimSpace(l); l != "" {
			out = append(out, l)
		}
	}
	return out
}

func dedupeStrings(in []string) []string {
	seen := make(map[string]bool, len(in))
	out := in[:0:0]
	for _, s := range in {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}
