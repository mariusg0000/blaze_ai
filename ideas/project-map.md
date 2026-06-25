# Project Map

## Goal

Create a `project-map` skill that scans the current working folder and generates a Markdown map of the relevant project structure.

## Core Behavior

The generated file should present folders and important files in a tree-like Markdown structure. Each documented item gets one or two short sentences explaining its role, with more detail near important areas and less detail in repetitive or low-value areas.

## Prompt Integration

At every prompt build, the runtime should look for this project map file in the working folder. If the file exists, inject it automatically into the prompt as project-local context.

## Filtering Rules

The skill must avoid documenting noisy or low-value files one by one. For example, if a folder contains many small assets such as fonts, icons, or generated fragments, the map should describe the folder and skip the individual files unless a specific file is unusually important.

## Open Design Points

- Where the generated file should live and what fixed filename it should use.
- How the skill decides detail depth for each subtree.
- Which folders and file patterns are globally ignored by default.
- Whether regeneration is always full or can be incremental.
