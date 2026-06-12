package direct

// ReviewToolsConfig configures the review tool set.
type ReviewToolsConfig struct {
	// Dir is the repository working directory; all file tools are sandboxed to it.
	Dir string
	// DiffBase / DiffHead are the default git_diff range (target / source branch).
	DiffBase string
	DiffHead string
	// PreloadedPaths are files already shown in the kickoff (pre-loaded changed
	// files); read-dedup is seeded with them so the model is not re-served their
	// content.
	PreloadedPaths []string
}

// NewReviewRegistry builds the narrow review tool set: read_file, read_files,
// glob, grep, git_diff and the terminal submit_review.
func NewReviewRegistry(cfg ReviewToolsConfig) *Registry {
	reg := NewRegistry()
	rt := newReadTracker(cfg.PreloadedPaths)
	reg.Register(readFileTool(cfg.Dir, rt))
	reg.Register(readFilesTool(cfg.Dir, rt))
	reg.Register(globTool(cfg.Dir))
	reg.Register(grepTool(cfg.Dir))
	reg.Register(gitDiffTool(cfg.Dir, cfg.DiffBase, cfg.DiffHead))
	// AST-index navigation tools — only when the binary is available; otherwise
	// the model stays on grep/read (graceful degradation).
	if astIndexAvailable() {
		registerAstTools(reg, cfg.Dir, cfg.DiffBase)
	}
	// Review output: streamed in small pieces (set_group ×5, add_issues) and
	// finalized by submit_review — a monolithic payload overflows small models'
	// output cap and arrives as truncated JSON.
	b := newReviewBuilder()
	reg.Register(setGroupTool(b))
	reg.Register(addIssuesTool(b))
	reg.Register(submitReviewTool(cfg.Dir, b, reg))
	return reg
}
