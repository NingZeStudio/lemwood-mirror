package lemwood_mirror

import "embed"

//go:embed all:web/default all:web/admin
var EmbeddedFiles embed.FS
