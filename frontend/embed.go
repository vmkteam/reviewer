package frontend

import "embed"

//go:embed dist/*
var DistFS embed.FS

//go:embed dist-vt/*
var DistVTFS embed.FS
