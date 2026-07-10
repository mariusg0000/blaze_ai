## Feature Description
Add a favorite models list to the runtime config with Ctrl+\ cycling and /model +/- management commands.

## Rationale And Implementation
The user wanted fast model switching between a curated subset of models without navigating the interactive /model menu each time. Implemented as a persistent favorite_models list in config.json with cyclic wrap-around on Ctrl+\, slash-command add/remove via /model + and /model -, and a smart switch-line display that overwrites the same terminal line on consecutive switches.

## Modified Files
- internal/config/config.go: Add AddFavorite and RemoveFavorite methods with validation and persistence
- internal/config/config_test.go: Unit tests for AddFavorite (valid, duplicate, invalid format, missing provider) and RemoveFavorite (found, not found, last item)
- internal/console/console.go: model_switch event handler, /model + and /model - subcommands, switchLineActive/switchLineWidth tracking, writeSwitchStatus method for overwriting the previous switch line, Shortcuts section in splash
- internal/console/reader.go: Ctrl+\ (0x1C) detection returning "model_switch" event
- internal/runtime/runtime.go: NextFavoriteModel method for cyclic model switching through FavoriteModels with wrap-around
- internal/runtime/runtime_test.go: Unit tests for NextFavoriteModel (normal cycle, empty list, single item, current not in list)
- main.go: signal.Ignore(syscall.SIGQUIT) to prevent Ctrl+\ from terminating the process outside raw mode
