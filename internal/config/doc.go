// config/doc.go — BlazeAI runtime configuration loading and validation.
// Loads config.json from app_home/config/, validates providers, models, role assignments,
// and compaction thresholds. Triggers first-run setup when config is missing or default role is unassigned.
// Layer: configuration. Dependencies: internal/platform (app home path resolution).
package config
