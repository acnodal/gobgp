# GoBGP Development Tools

This directory contains helper scripts for GoBGP development.

## Protocol Buffer Generation

### generate-proto.sh

Regenerates all Protocol Buffer files from `.proto` definitions.

**Usage:**
```bash
./tools/generate-proto.sh
```

**What it does:**
- Checks for required tools (`protoc`, `protoc-gen-go`, `protoc-gen-go-grpc`)
- Automatically installs missing Go plugins if needed
- Generates `.pb.go` and `_grpc.pb.go` files in the `api/` directory
- Uses correct import paths and source relative paths

**When to use:**
- After modifying any `.proto` files in `proto/api/`
- When updating gRPC API definitions
- After pulling changes that modify proto files

**Requirements:**
- `protoc` (Protocol Buffer compiler)
  - Debian/Ubuntu: `sudo apt-get install protobuf-compiler`
  - macOS: `brew install protobuf`
  - Or download from https://github.com/protocolbuffers/protobuf/releases

- `protoc-gen-go` and `protoc-gen-go-grpc` (installed automatically by script)

**Example output:**
```
=== GoBGP Protocol Buffer Generation ===
Project root: /home/user/go/gobgp

Installed tools:
  protoc version: libprotoc 3.21.12
  protoc-gen-go: /home/user/go/bin/protoc-gen-go
  protoc-gen-go-grpc: /home/user/go/bin/protoc-gen-go-grpc

Generating protobuf files...

Generated files:
  api/attribute.pb.go (181K)
  api/capability.pb.go (43K)
  api/common.pb.go (18K)
  api/extcom.pb.go (63K)
  api/gobgp_grpc.pb.go (110K)
  api/gobgp.pb.go (479K)
  api/nlri.pb.go (107K)

=== Protocol Buffer Generation Complete ===
```

**Troubleshooting:**

If you get "protoc not found":
```bash
# Debian/Ubuntu
sudo apt-get install protobuf-compiler

# macOS
brew install protobuf
```

If Go plugins are not in PATH, the script will attempt to install them automatically.
Make sure `$GOPATH/bin` or `$HOME/go/bin` is in your PATH for future runs.

## Other Tools

This directory may contain other development tools. Check individual files for documentation.
