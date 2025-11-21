# Phase 4: Create Production Fork - Summary

**Status**: Ready to Execute
**Estimated Time**: 2-3 hours
**Prerequisites**: All previous phases complete ✅

## Overview

Phase 4 creates the production fork `purelb/gobgp-netlink` with proper versioning, releases, and binaries.

## Versioning Strategy

### Dual Tagging Approach

We use **two tags** for each release:

1. **Fork Version** (v1.x.x): Our release version following semantic versioning
2. **Upstream Marker** (v4.x.x): Tracks which GoBGP version we're based on

**Example for First Release**:
- `v1.0.0`: First production release of the fork
- `v4.0.0`: Based on upstream GoBGP v4.0.0

### Why Dual Tags?

1. **Clarity**: Clear distinction between fork versioning and upstream versioning
2. **Dependency Management**: Go modules can reference either tag
3. **Semantic Versioning**: Fork follows semver independently of upstream
4. **Upstream Tracking**: Easy to see which upstream version we're based on

### Version Display

```bash
$ ./gobgpd --version
gobgpd version PureLB-fork:01.00.00 [base: gobgp-4.0.0]
```

### Future Versions

| Fork Version | Upstream Base | Description |
|--------------|---------------|-------------|
| v1.0.0 | v4.0.0 | Initial release |
| v1.0.1 | v4.0.0 | Bug fix (same base) |
| v1.1.0 | v4.0.0 | Feature update (same base) |
| v2.0.0 | v4.1.0 | Major update (new base) |

## Execution Steps

### 1. Create Fork (GitHub UI)

Navigate to https://github.com/acnodal/gobgp and click "Fork":
- Organization: `purelb`
- Repository name: `gobgp-netlink`
- Description: "GoBGP with Linux netlink integration for Kubernetes load balancing"

### 2. Set Up Local Repository

```bash
cd /home/adamd/go/gobgp

# Add purelb remote
git remote add purelb https://github.com/purelb/gobgp-netlink.git

# Create main branch from master
git checkout master
git checkout -b main

# Push main branch
git push purelb main
```

Then via GitHub Settings:
- Set `main` as default branch
- Delete `master` branch (optional)

### 3. Create Tags

```bash
# Tag v1.0.0 (fork version)
git tag -a v1.0.0 -m "GoBGP-Netlink v1.0.0

First production release of GoBGP with Linux netlink integration.
Based on upstream GoBGP v4.0.0.

Features:
- Linux routing table import/export via netlink
- VRF-aware routing (IPv4 and IPv6)
- Connected route import from interfaces
- BGP route export to Linux routing tables
- Community-based export filtering
- Nexthop validation with ONLINK support
- Comprehensive CLI tools
- Statistics and observability

Platform: Linux-only (kernel 4.3+ recommended)

Documentation: docs/sources/netlink.md

🤖 Generated with Claude Code
Co-Authored-By: Claude <noreply@anthropic.com>"

# Tag v4.0.0 (upstream alignment)
git tag -a v4.0.0 -m "Alignment with upstream GoBGP v4.0.0

This tag marks alignment with upstream osrg/gobgp v4.0.0.
For PureLB fork features, see v1.0.0.

🤖 Generated with Claude Code
Co-Authored-By: Claude <noreply@anthropic.com>"

# Push both tags
git push purelb v1.0.0
git push purelb v4.0.0
```

### 4. Build Release Binaries

```bash
# Build for Linux amd64
GOOS=linux GOARCH=amd64 go build -o gobgpd-linux-amd64 \
  -ldflags "-X github.com/osrg/gobgp/v4/internal/pkg/version.COMMIT=$(git rev-parse --short HEAD)" \
  ./cmd/gobgpd

GOOS=linux GOARCH=amd64 go build -o gobgp-linux-amd64 \
  -ldflags "-X github.com/osrg/gobgp/v4/internal/pkg/version.COMMIT=$(git rev-parse --short HEAD)" \
  ./cmd/gobgp

# Build for Linux arm64
GOOS=linux GOARCH=arm64 go build -o gobgpd-linux-arm64 \
  -ldflags "-X github.com/osrg/gobgp/v4/internal/pkg/version.COMMIT=$(git rev-parse --short HEAD)" \
  ./cmd/gobgpd

GOOS=linux GOARCH=arm64 go build -o gobgp-linux-arm64 \
  -ldflags "-X github.com/osrg/gobgp/v4/internal/pkg/version.COMMIT=$(git rev-parse --short HEAD)" \
  ./cmd/gobgp

# Verify binaries
file gobgpd-linux-* gobgp-linux-*
ls -lh gobgpd-linux-* gobgp-linux-*
```

### 5. Create GitHub Releases

#### Release 1: v1.0.0 (Primary Release)

```bash
gh release create v1.0.0 \
  --repo purelb/gobgp-netlink \
  --title "GoBGP-Netlink v1.0.0" \
  --notes "# GoBGP-Netlink v1.0.0

First production release of GoBGP with Linux netlink integration.

## Version Information
- **Fork Version**: v1.0.0
- **Based on**: GoBGP v4.0.0 (also tagged as \`v4.0.0\`)
- **Platform**: Linux-only

## Features
- ✅ Import connected routes from Linux interfaces into BGP RIB
- ✅ Export BGP routes to Linux routing tables
- ✅ Full VRF (Virtual Routing and Forwarding) support
- ✅ IPv4 and IPv6 dual-stack support
- ✅ Community-based export filtering
- ✅ Nexthop validation with ONLINK support
- ✅ CLI tools: \`gobgp netlink\` commands
- ✅ Statistics and observability

## Documentation
- [Netlink Integration Guide](https://github.com/purelb/gobgp-netlink/blob/main/docs/sources/netlink.md)
- [Configuration Examples](https://github.com/purelb/gobgp-netlink/blob/main/docs/sources/configuration.md)
- [Test Coverage Analysis](https://github.com/purelb/gobgp-netlink/blob/main/docs/TEST_COVERAGE_ANALYSIS.md)
- [Smoke Test Results](https://github.com/purelb/gobgp-netlink/blob/main/docs/SMOKE_TEST_RESULTS.md)

## Platform Requirements
⚠️ **Linux-only**: Uses vishvananda/netlink library
- Kernel 4.3+ (for VRF support)
- Root or CAP_NET_ADMIN capability

## Installation
Download the appropriate binary for your architecture:
- \`gobgpd-linux-amd64\` / \`gobgp-linux-amd64\` for x86_64 systems
- \`gobgpd-linux-arm64\` / \`gobgp-linux-arm64\` for ARM64 systems

Make executable:
\`\`\`bash
chmod +x gobgpd-linux-amd64 gobgp-linux-amd64
sudo mv gobgpd-linux-amd64 /usr/local/bin/gobgpd
sudo mv gobgp-linux-amd64 /usr/local/bin/gobgp
\`\`\`

## Testing
All 16 automated smoke tests passing (100%). See [SMOKE_TEST_RESULTS.md](https://github.com/purelb/gobgp-netlink/blob/main/docs/SMOKE_TEST_RESULTS.md).

## Known Issues
- VRF export with same-subnet nexthop may fail (use ONLINK with validate-nexthop=false)

## Acknowledgments
- Original GoBGP: [osrg/gobgp](https://github.com/osrg/gobgp)
- Maintained by: PureLB Kubernetes Load Balancer Team" \
  gobgpd-linux-amd64 \
  gobgpd-linux-arm64 \
  gobgp-linux-amd64 \
  gobgp-linux-arm64
```

#### Release 2: v4.0.0 (Upstream Alignment Marker)

```bash
gh release create v4.0.0 \
  --repo purelb/gobgp-netlink \
  --title "Upstream Alignment: GoBGP v4.0.0" \
  --notes "# Upstream GoBGP v4.0.0 Alignment

This tag marks alignment with upstream osrg/gobgp v4.0.0.

**For PureLB fork-specific features and releases, see the v1.x.x release series.**

This tag exists for:
- Dependency management (Go modules referencing base version)
- Clear upstream version tracking
- Semantic versioning clarity

Primary release: [v1.0.0](https://github.com/purelb/gobgp-netlink/releases/tag/v1.0.0)

## Changes from Upstream v4.0.0
See [v1.0.0 release notes](https://github.com/purelb/gobgp-netlink/releases/tag/v1.0.0) for netlink feature additions."
```

### 6. Verify Setup

Checklist:
- [ ] Repository created: https://github.com/purelb/gobgp-netlink
- [ ] `main` is default branch
- [ ] Tags created: `v1.0.0`, `v4.0.0`
- [ ] Release `v1.0.0` has 4 binary attachments
- [ ] Release `v4.0.0` references `v1.0.0`
- [ ] README shows PureLB branding
- [ ] Version in binary: `PureLB-fork:01.00.00 [base: gobgp-4.0.0]`

### 7. Post-Release Tasks

#### Update Repository Settings

- **Description**: "GoBGP with Linux netlink integration for Kubernetes load balancing"
- **Website**: https://purelb.io
- **Topics**: `bgp`, `networking`, `kubernetes`, `load-balancer`, `netlink`, `vrf`, `linux`
- **Enable Issues**: Yes
- **Enable Discussions**: Optional

#### Announce Release

Post in:
- PureLB Slack: https://kubernetes.slack.com/archives/C01BCB7U031
- GitHub Discussion (if enabled)

Message template:
```
🎉 GoBGP-Netlink v1.0.0 Released!

We're excited to announce the first production release of GoBGP-Netlink,
a fork of GoBGP with comprehensive Linux netlink integration.

Features:
• Import/export routes between BGP and Linux routing tables
• Full VRF support for multi-tenant environments
• IPv4/IPv6 dual-stack
• Community-based filtering
• Kubernetes-ready

Based on GoBGP v4.0.0
Docs: https://github.com/purelb/gobgp-netlink
Release: https://github.com/purelb/gobgp-netlink/releases/tag/v1.0.0

Linux-only (kernel 4.3+ recommended)
```

## Files Modified

### Updated Files
- `internal/pkg/version/version.go`: Updated to MAJOR=4, FORK_MAJOR=1
- `docs/MERGE_PLAN.md`: Updated Phase 4 with dual tagging strategy

### New Files
- `docs/TEST_COVERAGE_ANALYSIS.md`: Comprehensive test coverage analysis
- `docs/PHASE_4_SUMMARY.md`: This file

## Success Criteria

- [x] Version numbers updated (MAJOR=4, FORK_MAJOR=1)
- [x] Version display shows correct format
- [x] Phase 4 plan documented with dual tagging
- [x] Test coverage analyzed and documented
- [ ] Fork created at purelb/gobgp-netlink
- [ ] `main` branch set as default
- [ ] Both tags (v1.0.0, v4.0.0) pushed
- [ ] Both releases created with proper notes
- [ ] Binaries built and attached to v1.0.0
- [ ] Repository settings configured
- [ ] Release announced

## Timeline

- **Fork Creation**: 15 minutes
- **Branch Setup**: 10 minutes
- **Tag Creation**: 10 minutes
- **Binary Build**: 30 minutes (cross-compilation)
- **Release Creation**: 20 minutes
- **Verification**: 15 minutes
- **Announcement**: 10 minutes

**Total**: ~2 hours

## Rollback

If issues arise:
1. Delete fork repository (all data remains in acnodal/gobgp)
2. Fix issues
3. Restart Phase 4

No impact on development repository.

## Next Steps After Phase 4

1. **Monitor**: Watch for GitHub issues/PRs
2. **Docker Images**: Consider containerized distribution
3. **PureLB Integration**: Update PureLB to use new fork
4. **Documentation**: Add more examples and guides
5. **Upstream Sync**: Establish process for pulling upstream updates

---

**Ready to Execute**: Yes ✅
**Prerequisites Met**: All phases 1-3 complete, tests passing, documentation ready
**Risk Level**: Low (fork is copy, no impact on development)
