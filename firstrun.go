// firstrun.go — interactive first-run setup for BlazeAI.
// Prompts the user to select a provider, enter an API key, retrieve models from the endpoint,
// and assign model roles. Writes the completed config to disk.
// Layer: application entry. Dependencies: internal/config, internal/provider, internal/platform.
package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"

	"blazeai/internal/config"
	"blazeai/internal/platform"
	"blazeai/internal/provider"
)

// knownProviders is the curated list of OpenAI-compatible providers for first-run setup.
// Maximum 15 entries per spec.
var knownProviders = []config.Provider{
	{Name: "openrouter", Endpoint: "https://openrouter.ai/api/v1"},
	{Name: "deepseek", Endpoint: "https://api.deepseek.com/v1"},
	{Name: "openai", Endpoint: "https://api.openai.com/v1"},
	{Name: "groq", Endpoint: "https://api.groq.com/openai/v1"},
	{Name: "anthropic", Endpoint: "https://api.anthropic.com/v1"},
	{Name: "together", Endpoint: "https://api.together.xyz/v1"},
	{Name: "mistral", Endpoint: "https://api.mistral.ai/v1"},
	{Name: "perplexity", Endpoint: "https://api.perplexity.ai"},
	{Name: "fireworks", Endpoint: "https://api.fireworks.ai/inference/v1"},
	{Name: "cohere", Endpoint: "https://api.cohere.ai/v1"},
	{Name: "xai", Endpoint: "https://api.x.ai/v1"},
	{Name: "hyperbolic", Endpoint: "https://api.hyperbolic.xyz/v1"},
	{Name: "infermatic", Endpoint: "https://api.infermatic.ai/v1"},
	{Name: "opencode-go", Endpoint: "https://opencode.ai/zen/go/v1"},
	{Name: "lmstudio", Endpoint: "http://localhost:1234/v1"},
}

// firstRun executes the interactive first-run setup.
// Creates a config with at least one provider and a default model role.
//
// WHAT:  Interactive setup that guides the user through provider/key/model selection.
// WHY:   Required when config is missing or default role is unassigned.
// HOW:   Console prompts for provider selection, API key, model retrieval, role assignment.
// PARAMS: out — output writer; reader — buffered input reader.
// RETURNS: *config.Config — completed config; error if any step fails fatally.
func firstRun(out io.Writer, reader *bufio.Reader) (*config.Config, error) {

	fmt.Fprintln(out, "BlazeAI first-run setup")
	fmt.Fprintln(out, strings.Repeat("-", 40))
	fmt.Fprintln(out)

	// Step 1: provider selection
	p, err := selectProvider(out, reader)
	if err != nil {
		return nil, err
	}

	// Step 2: API key entry
	fmt.Fprintf(out, "Enter API key for %s: ", p.Name)
	key, err := reader.ReadString('\n')
	if err != nil {
		return nil, fmt.Errorf("cannot read API key: %w", err)
	}
	p.APIKey = strings.TrimSpace(key)
	if p.APIKey == "" {
		return nil, fmt.Errorf("API key cannot be empty")
	}

	// Step 3: model retrieval
	fmt.Fprintln(out)
	fmt.Fprintln(out, "Retrieving models from provider...")
	client := provider.NewClientRaw(p.Endpoint, p.APIKey)
	models, err := client.ListModels()
	if err != nil {
		return nil, fmt.Errorf("cannot retrieve models: %w", err)
	}
	if len(models) == 0 {
		return nil, fmt.Errorf("provider returned no models")
	}
	sort.Strings(models)

	// Step 4: model selection
	modelID, err := selectModel(out, reader, models, p.Name)
	if err != nil {
		return nil, err
	}

	// Step 5: build config
	cfg := config.Default()
	cfg.Providers = []config.Provider{p}
	cfg.FavoriteModels = []string{modelID}
	cfg.Roles.Default = modelID

	// Step 6: optional roles
	if err := assignOptionalRoles(out, reader, models, p.Name, cfg); err != nil {
		return nil, err
	}

	cfg.LastModel = modelID

	// Save config (without modes — modes go to separate file)
	if err := cfg.Save(); err != nil {
		return nil, fmt.Errorf("cannot save config: %w", err)
	}

	// Save default work mode to modes.json
	modes := config.DefaultMode(modelID)
	if err := modes.Save(); err != nil {
		return nil, fmt.Errorf("cannot save modes: %w", err)
	}

	fmt.Fprintln(out)
	fmt.Fprintf(out, "Config saved. Default model: %s\n", modelID)
	return cfg, nil
}

// selectProvider presents the curated provider list and reads the user's choice.
//
// WHAT:  Displays known providers and a custom option, reads selection.
// PARAMS: out — output writer; reader — buffered input reader.
// RETURNS: config.Provider — selected provider with endpoint; error if selection is invalid.
func selectProvider(out io.Writer, reader *bufio.Reader) (config.Provider, error) {
	fmt.Fprintln(out, "Select a provider:")
	for i, p := range knownProviders {
		fmt.Fprintf(out, "  %2d. %s (%s)\n", i+1, p.Name, p.Endpoint)
	}
	fmt.Fprintf(out, "  %2d. Custom (enter name, endpoint, API key manually)\n", len(knownProviders)+1)
	fmt.Fprintln(out)
	fmt.Fprint(out, "Enter number: ")
	input, err := reader.ReadString('\n')
	if err != nil {
		return config.Provider{}, fmt.Errorf("cannot read selection: %w", err)
	}
	input = strings.TrimSpace(input)
	num, err := strconv.Atoi(input)
	if err != nil {
		return config.Provider{}, fmt.Errorf("invalid number: %s", input)
	}

	if num >= 1 && num <= len(knownProviders) {
		return knownProviders[num-1], nil
	}

	if num == len(knownProviders)+1 {
		return customProvider(out, reader)
	}

	return config.Provider{}, fmt.Errorf("number out of range: %d", num)
}

// customProvider prompts for manual provider details.
//
// WHAT:  Reads a custom provider name and endpoint from the user.
// PARAMS: out — output writer; reader — buffered input reader.
// RETURNS: config.Provider — user-defined provider; error if input is invalid.
func customProvider(out io.Writer, reader *bufio.Reader) (config.Provider, error) {
	fmt.Fprintln(out, "Custom provider setup:")
	fmt.Fprint(out, "  Provider name: ")
	name, err := reader.ReadString('\n')
	if err != nil {
		return config.Provider{}, fmt.Errorf("cannot read name: %w", err)
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return config.Provider{}, fmt.Errorf("provider name cannot be empty")
	}

	fmt.Fprint(out, "  Endpoint URL: ")
	endpoint, err := reader.ReadString('\n')
	if err != nil {
		return config.Provider{}, fmt.Errorf("cannot read endpoint: %w", err)
	}
	endpoint = strings.TrimSpace(endpoint)
	if endpoint == "" {
		return config.Provider{}, fmt.Errorf("endpoint cannot be empty")
	}

	return config.Provider{Name: name, Endpoint: endpoint}, nil
}

// selectModel presents the retrieved model list and reads the user's choice.
//
// WHAT:  Displays available models and reads selection.
// PARAMS: out — output; reader — input; models — list of model IDs; providerName — for the full ID.
// RETURNS: string — full provider/model_name identifier; error if selection is invalid.
func selectModel(out io.Writer, reader *bufio.Reader, models []string, providerName string) (string, error) {
	fmt.Fprintln(out)
	fmt.Fprintln(out, "Available models:")
	for i, m := range models {
		fmt.Fprintf(out, "  %2d. %s\n", i+1, m)
	}
	fmt.Fprintln(out)
	fmt.Fprint(out, "Select default model number: ")
	input, err := reader.ReadString('\n')
	if err != nil {
		return "", fmt.Errorf("cannot read selection: %w", err)
	}
	input = strings.TrimSpace(input)
	num, err := strconv.Atoi(input)
	if err != nil {
		return "", fmt.Errorf("invalid number: %s", input)
	}
	if num < 1 || num > len(models) {
		return "", fmt.Errorf("number out of range: %d", num)
	}
	return providerName + "/" + models[num-1], nil
}

// assignOptionalRoles prompts for optional vision and summarization role assignment.
//
// WHAT:  Asks the user if they want to assign vision and summarization models.
// PARAMS: out — output; reader — input; models — available models; providerName — for ID; cfg — config to update.
// RETURNS: error if input reading fails fatally.
func assignOptionalRoles(out io.Writer, reader *bufio.Reader, models []string, providerName string, cfg *config.Config) error {
	for _, role := range []string{"vision", "summarization"} {
		fmt.Fprintln(out)
		fmt.Fprintf(out, "Assign %s role? (y/N): ", role)
		input, err := reader.ReadString('\n')
		if err != nil {
			return fmt.Errorf("cannot read %s role: %w", role, err)
		}
		input = strings.TrimSpace(strings.ToLower(input))
		if input != "y" && input != "yes" {
			continue
		}
		fmt.Fprintln(out, "Available models:")
		for i, m := range models {
			fmt.Fprintf(out, "  %2d. %s\n", i+1, m)
		}
		fmt.Fprintf(out, "Select %s model number: ", role)
		input, err = reader.ReadString('\n')
		if err != nil {
			return fmt.Errorf("cannot read %s model: %w", role, err)
		}
		input = strings.TrimSpace(input)
		num, err := strconv.Atoi(input)
		if err != nil || num < 1 || num > len(models) {
			fmt.Fprintf(out, "Skipping %s role (invalid selection)\n", role)
			continue
		}
		modelID := providerName + "/" + models[num-1]
		switch role {
		case "vision":
			cfg.Roles.Vision = modelID
		case "summarization":
			cfg.Roles.Summarization = modelID
		}
		cfg.FavoriteModels = append(cfg.FavoriteModels, modelID)
	}
	return nil
}

// runFirstRun triggers first-run setup with the detected platform OS.
// Wrapper that uses os.Stdout and os.Stdin for I/O.
func runFirstRun() (*config.Config, error) {
	return firstRun(os.Stdout, bufio.NewReader(os.Stdin))
}

// platformOS resolves the current OS and returns a clear error if unsupported.
func platformOS() (platform.OS, error) {
	return platform.Detect()
}
