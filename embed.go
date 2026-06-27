// embed.go — Go embed directives for builtin assets (prompts and skills).
// Embeds the prompts/ and skills/ directories into the binary at compile time,
// making the application a single self-contained executable.
// Layer: application entry. Embedded in package main at project root.

package main

import "embed"

//go:embed prompts/*
var embeddedPrompts embed.FS

//go:embed skills
var embeddedBuiltinSkills embed.FS
