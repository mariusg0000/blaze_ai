// levels.go — standard reasoning level constants and valid-level registry.
// Defines the abstract reasoning effort levels used across all providers.
// Layer: pure domain constants. Dependencies: none.
package reasoning

// Standard reasoning effort levels in ascending order of compute intensity.
//
// WHAT:  Canonical level names shared across all provider descriptors.
// WHY:   Providers invent their own parameter values; a standard set prevents
//
//	reasoning-level logic from scattering across provider code.
const (
	LevelNone  = "none"
	LevelMin   = "min"
	LevelLow   = "low"
	LevelMed   = "med"
	LevelHigh  = "high"
	LevelXHigh = "xhigh"
	LevelMax   = "max"
)

// ValidLevels lists all standard reasoning levels in ascending order.
//
// WHAT:  The complete set of recognized reasoning levels.
// WHY:   Validation and iteration need a single source of truth.
var ValidLevels = []string{LevelNone, LevelMin, LevelLow, LevelMed, LevelHigh, LevelXHigh, LevelMax}

// IsValidLevel reports whether the given string is a recognized reasoning level.
//
// WHAT:  Fast membership check against the standard level set.
// WHY:   Normalize must reject unknown level strings without fallback.
// PARAMS: level — candidate level string.
// RETURNS: true if level matches one of the ValidLevels entries.
func IsValidLevel(level string) bool {
	for _, v := range ValidLevels {
		if v == level {
			return true
		}
	}
	return false
}
