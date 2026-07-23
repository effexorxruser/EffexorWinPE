package locales

import "embed"

// FS holds application UI string catalogs.
//
//go:embed ru-RU.json en-US.json
var FS embed.FS
