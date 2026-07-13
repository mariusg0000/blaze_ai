# TODO: Implement project map

## WHAT MUST BE DONE
Create a `project-map` skill that scans the current working folder and generates a concise Markdown tree of relevant folders and important files, then injects the map automatically at prompt build when present.

## WHY IT MUST BE DONE
The agent needs reusable project-local structure context without documenting every generated, repetitive, or low-value asset individually.

## HOW IT MUST BE DONE
Decide the fixed output filename and location, detail depth rules, globally ignored folders and patterns, and whether regeneration is full or incremental. Implement filtered scanning, role descriptions, and prompt integration.
