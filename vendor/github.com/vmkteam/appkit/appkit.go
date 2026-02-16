package appkit

import (
	"fmt"
	"net/http"
	"runtime/debug"
)

type contextKey string

// Version returns app version from VCS info.
func Version() string {
	result := "devel"
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return result
	}

	for _, v := range info.Settings {
		if v.Key == "vcs.revision" {
			result = v.Value
		}
	}

	if len(result) > 8 {
		result = result[:8]
	}

	return result
}

// NewInternalHeaders returns prepared headers for services.
func NewInternalHeaders(appName, version string) http.Header {
	h := http.Header{}
	h.Set("User-Agent", fmt.Sprintf("%s (Version:%s)", appName, version))
	h.Set("Platform", appName)
	h.Set("Version", version)
	return h
}
