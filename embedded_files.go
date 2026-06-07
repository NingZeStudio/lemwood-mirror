package lemwood_mirror

import "embed"

//go:embed web/dist/** web/admin/**
var EmbeddedFiles embed.FS
