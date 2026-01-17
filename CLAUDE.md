# Claude Code Guidelines for this Repository

## Critical Rules

### DO NOT

- **DO NOT create PRs to the upstream repository** (korotovsky/slack-mcp-server)
- **DO NOT publish to the upstream npm package** (slack-mcp-server)
- **DO NOT run GitHub Actions for npm publishing** without explicit user approval

### DO

- **ONLY manage `@kazuph/mcp-slack`** - this is the scoped package for this fork

### This is a Fork

This repository (`kazuph/mcp-slack`) is a fork of `korotovsky/slack-mcp-server`.

- All changes should stay within this fork
- Use local builds for testing: `go build -o ./build/slack-mcp-server ./cmd/slack-mcp-server`
- If npm publishing is needed, it must be to a scoped package (e.g., `@kazuph/mcp-slack`)

## Development

### Build

```bash
go build -o ./build/slack-mcp-server ./cmd/slack-mcp-server
```

### Test

```bash
go test ./...
```

### Integration Test (requires .env with SLACK_MCP_XOXP_TOKEN)

```bash
source .env && go test -tags=integration -v ./pkg/text/...
```
