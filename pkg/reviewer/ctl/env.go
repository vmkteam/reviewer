package ctl

import (
	"net/mail"
	"os"
	"strconv"
)

// EnvDefault returns the value of env var key, or fallback when unset/empty.
func EnvDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// EnvBool parses an env var via strconv.ParseBool so common CI conventions
// (1/0/t/f/TRUE/FALSE/True/False) all work. Falls back when the var is unset
// or unparseable rather than guessing — bad input is more actionable as a
// no-op-with-default than a silent flip.
func EnvBool(key string, fallback bool) bool {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return fallback
	}
	return b
}

// AuthorName extracts the display name from "Name <email>" (CI_COMMIT_AUTHOR
// format) so the email isn't leaked into Slack notifications and the public
// API. Returns the input unchanged for plain logins or unparsable values.
func AuthorName(s string) string {
	addr, err := mail.ParseAddress(s)
	if err != nil || addr.Name == "" {
		return s
	}
	return addr.Name
}
