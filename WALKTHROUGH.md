# Walkthrough: Synchronizing Documentation for AI Agent Usability

I've completed the synchronization of the project documentation with the actual Go implementation. This ensures that an AI Agent connecting to this MCP server can correctly identify and use the available tools.

## Changes Made

### Documentation Consistency

- [x] **Updated [TOOLS.md](TOOLS.md)**
  - Switched the tool prefix from `opencode.` to `workspace.` to match `internal/mcp/register_tools.go`.
  - Removed obsolete/non-implemented tools (`search_code`, `search_symbols`, `get_file_context`, `run_build`).
  - Added full descriptions for the actual toolset: `read_file`, `write_file`, `inspect_workspace`, `read_code_fragment`, `apply_unified_diff`, `search_and_replace`, `secure_exec`, and `health`.
  - Aligned parameter names (e.g., `maxBytes` vs `max_bytes`, `allowCreate` vs `allow_create`) with the Go structs.

- [x] **Updated [AGENT.md](AGENT.md)**
  - Fixed the tool quick reference table and prefix.
  - Refined the "Typical Coding Loop" and "Traps" sections to use the correct tool names and parameters.
  - Optimized descriptions for better AI Agent understanding.

- [x] **Updated [README.md](README.md)**
  - Synchronized the tool overview table.
  - Updated the setup examples and integration config snippets (Claude/Cursor) to use the correct binary name and tool prefix.
  - Removed stale environment variable references that didn't match the current configuration pattern.

## Verification Results

### Code Review Consistency
I've cross-referenced every documentation change with the ground truth in [register_tools.go](internal/mcp/register_tools.go).

| Documentation | Tool Name | Go Implementation Status |
|---------------|-----------|--------------------------|
| [TOOLS.md](TOOLS.md) | `workspace.read_file` | Matches [L19](internal/mcp/register_tools.go#L19) |
| [TOOLS.md](TOOLS.md) | `workspace.write_file` | Matches [L37](internal/mcp/register_tools.go#L37) |
| [TOOLS.md](TOOLS.md) | `workspace.inspect_workspace` | Matches [L66](internal/mcp/register_tools.go#L66) |
| ... | ... | ... |

### Manual Verification
The documentation is now a reliable source of truth for any AI Agent or developer using the tool.

> [!IMPORTANT]
> The AI Agent can now learn the correct tool names and parameters directly from the project's own documentation, fulfilling the requirement for the tool to be self-explanatory to agents.
