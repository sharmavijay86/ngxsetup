package ngxsetup

import "embed"

// Embedded contains the original repo assets needed at runtime.
//
//go:embed common/** conf.d/** docs/** extra/** nginx/**
var Embedded embed.FS
