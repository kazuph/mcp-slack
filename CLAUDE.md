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

## Publishing to npm (@kazuph/mcp-slack)

**IMPORTANT:** `make npm-publish` is for upstream (korotovsky) and CANNOT be used for @kazuph scope.

### Complete Publishing Steps

#### 1. Build all platform binaries

```bash
make npm-copy-binaries
```

This builds binaries for all platforms and copies them to `npm/slack-mcp-server-{os}-{arch}/bin/`.

#### 2. Update platform package.json files to @kazuph scope

Each platform package must be renamed from `slack-mcp-server-{os}-{arch}` to `@kazuph/mcp-slack-{os}-{arch}`.

Run this command to update all 6 platform packages:

```bash
for os_arch in darwin-amd64 darwin-arm64 linux-amd64 linux-arm64 windows-amd64 windows-arm64; do
  dir="npm/slack-mcp-server-${os_arch}"
  new_name="@kazuph/mcp-slack-${os_arch}"
  os=$(echo $os_arch | cut -d- -f1)
  arch=$(echo $os_arch | cut -d- -f2)

  echo "{
  \"name\": \"${new_name}\",
  \"version\": \"X.Y.Z\",
  \"description\": \"Model Context Protocol (MCP) server for Slack Workspaces. This integration supports both Stdio and SSE transports, proxy settings and does not require any permissions or bots being created or approved by Workspace admins\",
  \"os\": [\"${os}\"],
  \"cpu\": [\"${arch}\"],
  \"publishConfig\": {
    \"access\": \"public\"
  }
}" >| "${dir}/package.json"
done
```

**Replace `X.Y.Z` with the actual version number!**

#### 3. Publish all platform packages

```bash
for os_arch in darwin-amd64 darwin-arm64 linux-amd64 linux-arm64 windows-amd64 windows-arm64; do
  cd npm/slack-mcp-server-${os_arch} && npm publish --access public && cd ../..
done
```

#### 4. Update and publish main package

Update `npm/slack-mcp-server/package.json`:

```json
{
  "name": "@kazuph/mcp-slack",
  "version": "X.Y.Z",
  "bin": { "mcp-slack": "bin/index.js" },
  "optionalDependencies": {
    "@kazuph/mcp-slack-darwin-amd64": "X.Y.Z",
    "@kazuph/mcp-slack-darwin-arm64": "X.Y.Z",
    "@kazuph/mcp-slack-linux-amd64": "X.Y.Z",
    "@kazuph/mcp-slack-linux-arm64": "X.Y.Z",
    "@kazuph/mcp-slack-windows-amd64": "X.Y.Z",
    "@kazuph/mcp-slack-windows-arm64": "X.Y.Z"
  },
  "publishConfig": { "access": "public" }
}
```

Then copy README/LICENSE and publish:

```bash
cp README.md LICENSE npm/slack-mcp-server/
cd npm/slack-mcp-server && npm publish --access public
```

### Package Structure

- **Main package**: `@kazuph/mcp-slack` (depends on platform-specific packages)
- **Platform packages**: `@kazuph/mcp-slack-{darwin,linux,windows}-{amd64,arm64}` (contain actual binaries)

### Prerequisites

- `npm whoami` should return `kazuph`
- If not logged in: `npm login`
