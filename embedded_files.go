package lemwood_mirror

import "embed"

//go:embed web/default/** web/default_v2/** web/admin/**
var EmbeddedFiles embed.FS
