package reviewer

import (
	"strings"
	"testing"
)

// TestPromptReviewJSON_StrictSchemaInvariants pins the parts of the
// review.json prompt suffix that prevent Claude from drifting onto
// SonarQube-style severity (major/minor/trivial) or treating files[]
// as a list of changed files. If you change these strings, update the
// expectations here and confirm the new wording actually solved a real
// case in /v1/debug/storage/.
func TestPromptReviewJSON_StrictSchemaInvariants(t *testing.T) {
	expected := []string{
		// Strict schema header.
		"STRICT SCHEMA",

		// Allowed enum values must be spelled out so Claude can grep its own output.
		"critical, high, medium, low",
		"architecture, code, security, tests, operability",

		// Forbidden values — the alias trap that bit us in /v1/debug/storage/196236df23f8.
		"major | minor | trivial",

		// files[] must not be confused with diff file list.
		"files[] — это РОВНО 5 объектов",
		"path/kind/lines",

		// Self-check section.
		"обязательная самопроверка",
		"major→high",

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

// TestPromptReviewJSON_ExampleHasConcreteEnumValues checks that the
// JSON example uses real enum values, not "a | b | c" annotations that
// Claude tends to substitute with values from training data (e.g. "major").
func TestPromptReviewJSON_ExampleHasConcreteEnumValues(t *testing.T) {
	bannedPlaceholders := []string{
		`"reviewType": "architecture | code`,
		`"severity": "critical | high`,
		`"fileType": "architecture | code`,
	}

	for _, placeholder := range bannedPlaceholders {
		if strings.Contains(promptReviewJSON, placeholder) {
			t.Errorf("promptReviewJSON uses enum-style placeholder %q in JSON example — replace with one concrete value", placeholder)
		}
	}

	// Concrete examples must be present.
	concrete := []string{
		`"reviewType": "architecture"`,
		`"reviewType": "code"`,
		`"reviewType": "operability"`,
		`"severity": "high"`,
		`"fileType": "code"`,
	}
	for _, frag := range concrete {
		if !strings.Contains(promptReviewJSON, frag) {
			t.Errorf("promptReviewJSON missing concrete example: %q", frag)
		}
	}
}
