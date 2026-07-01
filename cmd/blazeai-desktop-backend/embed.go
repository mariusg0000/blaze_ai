// embed.go — embedded prompt and skill assets for the desktop backend binary.
// Bundles the runtime prompt templates and builtin skill templates directly into
// the desktop backend so Electron can start without depending on source files.
// Layer: desktop backend application entry. Dependencies: standard library only.
package main

import "embed"

//go:embed resources/prompts/*
var embeddedPrompts embed.FS

//go:embed resources/skills
var embeddedBuiltinSkills embed.FS
