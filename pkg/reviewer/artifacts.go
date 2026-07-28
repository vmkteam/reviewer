package reviewer

// Review-run artifact names, shared by every place that must treat them
// specially so the set stays in ONE place: the debug-bundle collector and
// pre-run cleaner (pkg/reviewer/ctl) and the direct runner's diff excludes and
// glob/grep walk skips (pkg/reviewer/direct). A leftover artifact from a
// previous run that slips through any of those channels is presented to the
// model as part of the codebase — review.html even anchors it on the PREVIOUS
// review's findings.

// ReviewArtifactFiles are the fixed-name files reviewctl and its runners write
// into the working tree during a run: the review.json draft and the runner
// session logs.
var ReviewArtifactFiles = []string{"review.json", "claude-output.json", "opencode-output.jsonl", "direct-output.jsonl"}

// ReviewArtifactHTML is rendered from the draft after upload. It is cleaned
// before a run and hidden from review tools like the files above, but
// deliberately kept out of debug bundles (it duplicates the uploaded review).
const ReviewArtifactHTML = "review.html"

// ReviewArtifactMDSuffix marks the R*.md review bodies written by runners
// (matched by suffix — the name stem varies per review type and branch).
const ReviewArtifactMDSuffix = ".ai.md"
