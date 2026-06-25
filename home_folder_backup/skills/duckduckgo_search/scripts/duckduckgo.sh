#!/bin/bash
# DuckDuckGo search wrapper cu delay anti-rate-limit
# Urmareste: set -e (fail on error)
# Folosire: duckduckgo.sh text -k "query" -m 5
#           duckduckgo.sh news -k "query" -m 3 -o json

DELAY=${DDGS_DELAY:-3}
sleep "$DELAY"
exec /home/marius/blazeai/scripts/venv/bin/ddgs "$@"
