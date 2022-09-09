package config_test

import "embed"

// Should be used only for tests
//
// embed involves faster execution
// as it prevents OS system calls

//go:embed samples/*.y*ml
var Samples embed.FS
