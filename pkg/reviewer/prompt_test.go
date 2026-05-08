package reviewer

import (
	"strings"
	"testing"
)

// TestPromptReviewJSON_StrictSchemaInvariants pins the parts of the
// review.json prompt suffix that prevent Claude from drifting onto
// SonarQube-style severity (major/minor/trivial) or inventing alternate
// root keys (branch/baseBranch/tasks/summary). If you change these strings,
// update the expectations here and confirm the new wording actually solved
// a real case in /v1/debug/storage/.
func TestPromptReviewJSON_StrictSchemaInvariants(t *testing.T) {
	expected := []string{
		// Skeleton-on-disk framing.
		"reviewctl уже положил на диск",
		"НЕ создавай файл заново",

		// Strict schema header.
		"STRICT SCHEMA",

		// Allowed enum values must be spelled out so the model can grep its own output.
		"critical, high, medium, low",
		"architecture, code, security, tests, operability",

		// Forbidden values — the alias trap that bit us in /v1/debug/storage/196236df23f8.
		"major | minor | trivial",

		// Self-check section + alias mapping.
		"обязательная самопроверка",
		"major→high",

		// Guards against the Run #2 drift (e388cdb97c5f) where the model
		// invented `branch` / `baseBranch` / `tasks` and nested issues into files[].
		"Никаких `branch`/`baseBranch`/`tasks`",
		"плоский массив. НЕ внутри `files[]`",

		// Field name guard against renaming to "category".
		"category",
		"issueType",
	}

	for _, want := range expected {
		if !strings.Contains(promptReviewJSON, want) {
			t.Errorf("promptReviewJSON missing required fragment: %q", want)
		}
	}
}

// TestPromptReviewJSON_NoDeadInstructions guards against re-introducing
// dead instructions: durationMs/modelInfo are overwritten on the server
// (pkg/reviewer/ctl/ctl.go), so any guidance asking Claude to compute
// them just wastes tokens and attention.
func TestPromptReviewJSON_NoDeadInstructions(t *testing.T) {
	forbidden := []string{
		"date +%s%3N", // duration measurement — server overwrites DurationMs
		"START_MS",    //
		"END_MS",      //
		"costUsd: рассчитай по формуле", // cost is server-side from runner result
		"опционально, обычно 15-30%",    //
	}

	for _, frag := range forbidden {
		if strings.Contains(promptReviewJSON, frag) {
			t.Errorf("promptReviewJSON contains dead instruction: %q (server overwrites this field)", frag)
		}
	}
}

// TestPromptReviewJSON_NoEnumPlaceholders checks that the JSON example
// doesn't fall back to "a | b | c" annotations that the model tends to
// substitute with values from training data (e.g. "major").
func TestPromptReviewJSON_NoEnumPlaceholders(t *testing.T) {
	bannedPlaceholders := []string{
		`"reviewType": "architecture | code`,
		`"severity": "critical | high`,
		`"fileType": "architecture | code`,
	}

	for _, placeholder := range bannedPlaceholders {
		if strings.Contains(promptReviewJSON, placeholder) {
			t.Errorf("promptReviewJSON uses enum-style placeholder %q in example — replace with one concrete value", placeholder)
		}
	}

	if !strings.Contains(promptReviewJSON, `"severity": "high"`) {
		t.Error("issue example must use a concrete severity value")
	}
	if !strings.Contains(promptReviewJSON, `"fileType": "code"`) {
		t.Error("issue example must use a concrete fileType value")
	}
}
