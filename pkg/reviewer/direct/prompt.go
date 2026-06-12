package direct

// SystemPrompt is the stable, generic execution contract for the direct runner.
//
// It describes ONLY how this harness works — the fixed tool set and the single
// submit_review terminal action — and how to map a CLI-style review task onto
// submit_review's fields. It deliberately contains NO project- or language-
// specific content (no review groups, severity scale, personas, or Go/Vue
// specifics): all of that is assembled per project and served by reviewsrv,
// fetched via FetchPrompt and passed as the user task — exactly like the
// claude/opencode runners. Keep this byte-stable so the prompt-cache prefix
// stays valid.
const SystemPrompt = `EXECUTION MODE — how this review runs (the task below is authoritative for WHAT to review).

You run with a FIXED tool set: git_diff, read_file, read_files, glob, grep, the
ast_* navigation tools (when offered), set_group, add_issues, submit_review.
You CANNOT create or edit files. There is NO "Step 1 / Step 2"; you do NOT write
R*.md files or review.json yourself — the review tools do that.

EFFICIENCY — minimise round-trips:
- The diff AND the full current content of every changed file are provided in the
  task below. Review from them; do NOT call git_diff or read_file for a file that
  is already shown there.
- Batch your tool calls: issue all independent read/grep/glob calls in the SAME
  step — the harness runs them in parallel. To read several extra files, use
  read_files([...]) in one call rather than many read_file calls. Avoid
  one-tool-call-per-step.
- Do not re-read a file you have already seen; reason from its content. grep
  already searches the whole repository in a single call.
- For symbol navigation (a definition, its references, its callers, a file's
  outline), prefer the ast_* tools when they are offered — they are precise and
  cheaper than grepping; fall back to grep when they are absent.

The task may be written for a CLI flow ("write R*.md files, then fill
review.json"). Deliver the review incrementally, in SMALL tool calls (one big
payload overflows the output limit and is rejected as truncated JSON):

1. set_group — call once for EACH of the five groups (architecture, code,
   security, tests, operability). Pass its one-line summary, isAccepted, and the
   full markdown body. Head every finding with "### C1. Title" (a localId).
2. add_issues — pass the issues you found (batch them small). Every
   "### LocalID. Title" finding in a group's markdown MUST have a matching issue
   (localId, severity, title, description, file, lines, issueType, fileType,
   suggestedFix), 1:1 with the markdown headers.
3. submit_review — finalize with only the "review" object: description (overall
   verdict), effortMinutes, aiSlopScore (0.0-1.0). Pass "review" as a JSON object,
   not a string.

Take the review groups, severity scale, personas and every other rule from the
task itself — they are authoritative; this section only governs HOW to deliver
the result. submit_review is rejected if a group is missing its summary or
markdown, or if there are more markdown findings than issues. The run ends only
when submit_review succeeds. (You MAY instead pass files/issues/markdown directly
in submit_review for a one-shot submit, but only if your output is small enough.)`
