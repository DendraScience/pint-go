package defs

import "embed"

// Files contains the Pint definition texts vendored verbatim.
//
//go:embed default_en.txt constants_en.txt
var Files embed.FS

const (
	DefaultFile    = "default_en.txt"
	ConstantsFile  = "constants_en.txt"
	PintImportName = "constants_en.txt"
)
