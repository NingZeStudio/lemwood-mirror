package lemwood_mirror

import "embed"

//go:embed web/default/index.html web/admin/**
var EmbeddedFiles embed.FS
