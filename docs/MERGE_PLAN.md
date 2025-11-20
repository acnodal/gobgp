# Netlink Feature Merge Plan

**Created**: 2025-11-19
**Timeline**: 5-6 days
**Target Version**: v1.0.0-netlink (based on GoBGP v4.0.0)
**Platform**: Linux-only deployment
**GitHub Issue**: https://github.com/acnodal/gobgp/issues/2

## Overview

This document outlines the plan to merge the netlink feature branch into the main acnodal/gobgp repository and create a production fork (purelb/gobgp-netlink).

## Feature Summary

The netlink branch adds comprehensive Linux routing table integration to GoBGP with full VRF support:

### Import Features
- Import connected routes from Linux interfaces to GoBGP RIB
- VRF-aware import with per-VRF interface configuration
- IPv4 and IPv6 support
- Automatic rescan when VRFs are added/modified
- Statistics tracking (imported, withdrawn, errors)

### Export Features
- Export BGP routes to Linux routing tables
- VRF-aware export with per-VRF routing tables
- Community-based filtering (standard and large communities)
- Nexthop validation with ONLINK support for unreachable nexthops
- IPv4 and IPv6 support (full VPN NLRI handling)
- Route dampening to prevent flapping
- Statistics tracking (exported, withdrawn, validation metrics)

### CLI Commands
```bash
gobgp netlink                    # Show netlink status
gobgp netlink import             # Show import configuration
gobgp netlink import stats       # Show import statistics
gobgp netlink export             # List exported routes
gobgp netlink export --vrf NAME  # Filter by VRF
gobgp netlink export rules       # Show export rules
gobgp netlink export stats       # Show export statistics
gobgp netlink export flush       # Remove all exported routes
```

## Implementation Phases

### Phase 1: Merge to acnodal/gobgp master (4 hours)

#### Steps

1. **Update local netlink branch from master**
   ```bash
   cd /home/adamd/go/gobgp
   git checkout netlink
   git fetch origin
   git merge origin/master
   # Resolve any conflicts if they exist
   ```

2. **Rebase/clean commit history (optional)**
   ```bash
   # Optional: Clean up commit history for cleaner merge
   git rebase -i master
   ```

3. **Run tests to verify merge**
   ```bash
   # Unit tests
   go test ./pkg/server/...
   go test ./pkg/netlink/...

   # Build verification
   go build ./cmd/gobgpd
   go build ./cmd/gobgp

   # Check for obvious issues
   go vet ./...
   ```

4. **Merge to master**
   ```bash
   git checkout master
   git merge netlink --no-ff -m "Merge netlink feature with VRF support

   This adds comprehensive Linux routing table integration to GoBGP:

   Import features:
   - Import connected routes from Linux interfaces
   - VRF-aware import with per-VRF configuration
   - IPv4 and IPv6 support
   - Automatic VRF discovery and rescan

   Export features:
   - Export BGP routes to Linux routing tables
   - VRF-aware export with per-VRF tables
   - Community-based filtering
   - Nexthop validation with ONLINK support
   - Route dampening

   CLI:
   - gobgp netlink commands for status and statistics

   Platform: Linux-only (uses vishvananda/netlink)

   🤖 Generated with Claude Code
   Co-Authored-By: Claude <noreply@anthropic.com>"
   ```

5. **Push to acnodal/gobgp**
   ```bash
   git push origin master
   ```

#### Verification Checklist
- [ ] CI build succeeds (Linux builds)
- [ ] No merge conflicts
- [ ] All commits have proper messages
- [ ] Branch netlink can be safely archived/kept for reference

### Phase 2: Comprehensive Testing (2-3 days)

#### Test Environment Setup

**Requirements**:
- Linux VM or container with network namespace support
- Kernel 4.3+ (for VRF support)
- Root access or CAP_NET_ADMIN capability
- Multiple network interfaces (real or virtual)
- IPv4 and IPv6 addressing

**Sample Test Setup**:
```bash
# Create VRFs
ip link add vrf-blue type vrf table 10
ip link add vrf-red type vrf table 20
ip link set vrf-blue up
ip link set vrf-red up

# Create test interfaces
ip link add eth1 type dummy
ip link add eth2 type dummy
ip link set eth1 master vrf-blue
ip link set eth2 master vrf-red

# Add addresses
ip addr add 192.168.1.1/24 dev eth1
ip addr add 2001:db8:1::1/64 dev eth1
ip addr add 192.168.2.1/24 dev eth2
ip addr add 2001:db8:2::1/64 dev eth2

ip link set eth1 up
ip link set eth2 up
```

#### Test Matrix

##### 2.1 Import Testing (6-8 hours)

**Basic Import**:
- [ ] Import from single interface (global table)
- [ ] Import from multiple interfaces (global table)
- [ ] Verify routes appear in RIB: `gobgp global rib`
- [ ] Verify route attributes (origin, nexthop)
- [ ] Import statistics accuracy

**VRF Import**:
- [ ] Import to single VRF (single interface)
- [ ] Import to single VRF (multiple interfaces)
- [ ] Import to multiple VRFs simultaneously
- [ ] Verify routes in VRF RIB: `gobgp vrf <name> rib`
- [ ] VRF-specific statistics

**IPv4 and IPv6**:
- [ ] IPv4 route import
- [ ] IPv6 route import
- [ ] Mixed IPv4/IPv6 import to same VRF
- [ ] Verify address family handling

**Dynamic Changes**:
- [ ] Interface up/down handling
- [ ] Address add/remove on interface
- [ ] Routes withdrawn when interface goes down
- [ ] Routes re-added when interface comes up
- [ ] VRF added after GoBGP startup (rescan trigger)
- [ ] VRF deleted while GoBGP running

**Error Handling**:
- [ ] Non-existent interface in config
- [ ] Invalid interface name
- [ ] Permission denied (non-root test)
- [ ] Error statistics accuracy

##### 2.2 Export Testing (8-10 hours)

**Basic Export (Global Table)**:
- [ ] Export to global routing table (single rule)
- [ ] Export with multiple rules
- [ ] Verify routes in Linux: `ip route show`
- [ ] Verify protocol ID (201 or configured)
- [ ] Verify metric values

**VRF Export**:
- [ ] Export to single VRF table
- [ ] Export to multiple VRF tables
- [ ] Verify routes in VRF: `ip route show vrf <name>`
- [ ] VRF-specific routing table IDs
- [ ] VRF device LinkIndex for ONLINK routes

**Community Filtering**:
- [ ] Export with standard community filter
- [ ] Export with large community filter
- [ ] Export with multiple communities (AND logic)
- [ ] Export with no community filter (match all)
- [ ] Non-matching community (route not exported)

**IPv4 and IPv6**:
- [ ] IPv4 route export (unicast)
- [ ] IPv6 route export (unicast)
- [ ] IPv4 VPN route export (VRF)
- [ ] IPv6 VPN route export (VRF)
- [ ] Verify VPN NLRI handling (both LabeledVPNIPAddrPrefix and LabeledVPNIPv6AddrPrefix)

**Nexthop Validation**:
- [ ] Export with `validate-nexthop = true` (reachable nexthop)
- [ ] Export with `validate-nexthop = true` (unreachable nexthop - should fail)
- [ ] Export with `validate-nexthop = false` (ONLINK flag set)
- [ ] Verify ONLINK flag: `ip route show` (shows "onlink")
- [ ] VRF device LinkIndex set for ONLINK routes

**Route Lifecycle**:
- [ ] Route added to BGP, exported to Linux
- [ ] Route updated (metric change) in BGP, updated in Linux
- [ ] Route withdrawn from BGP, deleted from Linux
- [ ] Verify withdrawal statistics

**Stale Route Cleanup**:
- [ ] Export routes, restart GoBGP, verify old routes removed
- [ ] Cleanup from global routing table
- [ ] Cleanup from VRF routing tables
- [ ] Verify all VRF tables discovered and cleaned

**Error Handling**:
- [ ] Invalid nexthop handling
- [ ] VRF device not found
- [ ] Permission denied (non-root test)
- [ ] Duplicate route handling
- [ ] Malformed community strings
- [ ] Error statistics accuracy

##### 2.3 Combined Import/Export (4-6 hours)

**Integration Scenarios**:
- [ ] Import from eth0, export to vrf-blue routing table
- [ ] Import to vrf-blue, export to Linux vrf-blue table
- [ ] Re-advertisement of imported routes via BGP
- [ ] Verify no feedback loops (imported routes not re-exported)
- [ ] Statistics separation (import vs export counters)
- [ ] Combined IPv4 and IPv6 flows

##### 2.4 Scale Testing (4-6 hours)

**Route Scale**:
- [ ] Import 1000+ routes from interfaces
- [ ] Export 1000+ routes to Linux
- [ ] 10+ VRFs with import/export configured
- [ ] Measure convergence time
- [ ] Monitor memory usage
- [ ] Monitor CPU usage

**Performance Metrics**:
- Route import latency: ___ ms
- Route export latency: ___ ms
- Memory footprint: ___ MB (idle)
- Memory footprint: ___ MB (1000 routes)
- CPU usage: ___ % (steady state)

##### 2.5 Stability Testing (8-12 hours)

**Long-Running Tests**:
- [ ] Run for 24 hours with periodic route additions/deletions
- [ ] Monitor memory usage (check for leaks)
- [ ] Check error logs for unexpected issues
- [ ] Verify statistics consistency

**Restart and Reload**:
- [ ] Graceful shutdown and restart
- [ ] Stale route cleanup on restart
- [ ] Config reload without restart
- [ ] BGP session flap handling
- [ ] VRF add/delete during operation

**Memory Leak Detection**:
```bash
# Use Go's built-in profiling
gobgpd -f config.toml -p 6060 &
# Let run for hours, then:
go tool pprof http://localhost:6060/debug/pprof/heap
```

##### 2.6 CLI Testing (2-3 hours)

**Command Output Verification**:
- [ ] `gobgp netlink` - shows import/export status
- [ ] `gobgp netlink` - displays VRF imports correctly
- [ ] `gobgp netlink import` - shows detailed import config
- [ ] `gobgp netlink import stats` - shows accurate statistics
- [ ] `gobgp netlink export` - lists all exported routes
- [ ] `gobgp netlink export --vrf <name>` - filters by VRF
- [ ] `gobgp netlink export rules` - shows all export rules
- [ ] `gobgp netlink export rules` - shows VRF-specific rules
- [ ] `gobgp netlink export stats` - shows accurate statistics
- [ ] `gobgp netlink export flush` - removes all routes

**Output Format Validation**:
- Timestamps formatted correctly
- VRF names display ("global" for empty string)
- Interface lists formatted properly
- Statistics counters accurate

#### Test Results Documentation

Create `docs/NETLINK_TEST_RESULTS.md` with:
- Test environment details
- All test results (pass/fail)
- Performance metrics
- Issues found and resolutions
- Known limitations

### Phase 3: Documentation Updates (2 hours)

#### 3.1 Create docs/sources/netlink-integration.md

```markdown
# Netlink Integration

GoBGP supports bidirectional integration with the Linux routing table via netlink.

## Platform Requirements

- **Operating System**: Linux only
- **Kernel Version**: 4.3+ (for VRF support), 3.x (for basic features)
- **Permissions**: CAP_NET_ADMIN capability (typically requires root)
- **Dependencies**: vishvananda/netlink library (included)

## Architecture

### Import Flow
1. GoBGP scans configured interfaces for connected routes
2. Routes are converted to BGP paths with origin IGP
3. Paths are added to the appropriate RIB (global or VRF)
4. Routes are re-scanned every 5 seconds for changes

### Export Flow
1. BGP receives route update (from peer or policy)
2. Export rules are evaluated (community matching)
3. Nexthop validation performed (if enabled)
4. Route is added to Linux routing table via netlink
5. Exported routes tracked for withdrawal

## Configuration

### Import Configuration

**Global Import**:
```toml
[netlink.import]
  enabled = true
  vrf = ""  # Empty for global table, or VRF name
  interfaces = ["eth0", "eth1"]
```

**Per-VRF Import**:
```toml
[[vrfs]]
  [vrfs.config]
    name = "vrf-blue"
    rd = "64512:100"

  [vrfs.netlink-import]
    enabled = true
    interfaces = ["eth2", "eth3"]
```

### Export Configuration

**Global Export Rules**:
```toml
[[netlink.export.rules]]
  name = "rule1"
  vrf = ""  # Global routing table
  table-id = 0
  metric = 100
  validate-nexthop = true
  community-list = ["65001:100"]
  large-community-list = ["65001:1:100"]
```

**Per-VRF Export Rules**:
```toml
[[netlink.export.vrf-rules]]
  gobgp-vrf = "vrf-blue"
  linux-vrf = "vrf-blue"
  linux-table-id = 10
  metric = 100
  validate-nexthop = false  # Use ONLINK flag
  community-list = []  # Match all routes
```

### Configuration Options

**Import Options**:
- `enabled` (boolean): Enable netlink import
- `vrf` (string): Target VRF name (empty for global)
- `interfaces` (list): Interface names to scan

**Export Options**:
- `name` (string): Rule identifier
- `vrf` (string): Target VRF name (empty for global)
- `table-id` (integer): Linux routing table ID
- `metric` (integer): Route metric
- `validate-nexthop` (boolean): Validate nexthop reachability
- `community-list` (list): Standard communities to match
- `large-community-list` (list): Large communities to match

## CLI Commands

### Status and Configuration

```bash
# Show overall netlink status
gobgp netlink

# Show import configuration
gobgp netlink import

# Show export rules
gobgp netlink export rules
```

### Statistics

```bash
# Import statistics
gobgp netlink import stats

# Export statistics
gobgp netlink export stats
```

### Route Management

```bash
# List all exported routes
gobgp netlink export

# List exported routes for specific VRF
gobgp netlink export --vrf vrf-blue

# Remove all exported routes (cleanup)
gobgp netlink export flush
```

## How It Works

### Import Process

1. **Interface Scanning**:
   - Every 5 seconds, configured interfaces are scanned
   - Global unicast addresses are identified
   - Link-local and loopback addresses are ignored

2. **Route Creation**:
   - Network prefix calculated from IP and netmask
   - BGP path created with origin IGP, nexthop 0.0.0.0 (IPv4) or :: (IPv6)
   - Path marked as external (IsFromExternal = true)

3. **RIB Integration**:
   - Routes added to appropriate RIB (global or VRF)
   - Duplicate detection based on prefix
   - Withdrawal of routes no longer present

### Export Process

1. **Route Evaluation**:
   - BGP path update received
   - VPN vs unicast family determined
   - Appropriate export rules matched

2. **Community Filtering**:
   - If community lists specified, route must match ALL
   - Empty community list matches all routes
   - Both standard and large communities supported

3. **Nexthop Validation**:
   - If `validate-nexthop = true`, nexthop must be reachable
   - If `validate-nexthop = false`, ONLINK flag set (accept unreachable)
   - For VRF routes with ONLINK, VRF device LinkIndex set

4. **Route Installation**:
   - Route added to Linux via netlink with protocol 201
   - Tracked for future withdrawal
   - Statistics updated

## Troubleshooting

### Import Issues

**Routes not appearing in RIB**:
- Check interface names: `ip link show`
- Verify addresses on interfaces: `ip addr show`
- Check logs for errors: `gobgpd -f config.toml -t debug`
- Verify VRF exists: `ip vrf show`

**VRF import not working**:
- Ensure VRF created before GoBGP starts (or rescan triggered)
- Check VRF netlink-import enabled in config
- Verify interfaces assigned to VRF: `ip link show master <vrf-name>`

### Export Issues

**Routes not appearing in Linux**:
- Check export rules configured
- Verify community matching
- Check nexthop validation: `gobgp netlink export stats`
- Verify permissions (requires root or CAP_NET_ADMIN)
- Check Linux routing table: `ip route show table <table-id>`

**Routes with unreachable nexthop failing**:
- Set `validate-nexthop = false` in export rule
- Verify ONLINK flag set: `ip route show` (shows "onlink")

**IPv6 routes not exported**:
- Check IPv6 is enabled on interface
- Verify route family is RF_IPv6_UC or RF_IPv6_VPN
- Check VPN NLRI type handling in logs

### Permission Errors

```
Error: Operation not permitted
```
**Solution**: Run as root or with CAP_NET_ADMIN:
```bash
sudo gobgpd -f config.toml
# Or with capabilities:
sudo setcap cap_net_admin+ep /path/to/gobgpd
```

### Stale Routes After Restart

If routes with protocol 201 remain after restart:
- Check cleanup logic runs on startup
- Verify all VRF tables are discovered
- Manually clean: `ip route flush proto 201`

## Limitations

1. **Platform**: Linux only (uses vishvananda/netlink library)
2. **Permissions**: Requires CAP_NET_ADMIN (typically root)
3. **Import Frequency**: 5-second scan interval (not real-time)
4. **Route Protocol**: Exported routes use protocol 201 (configurable)
5. **Nexthop Handling**: 0.0.0.0 nexthops are rejected

## Examples

### Example 1: Simple Global Import/Export

```toml
[netlink.import]
  enabled = true
  vrf = ""
  interfaces = ["eth0"]

[[netlink.export.rules]]
  name = "default"
  vrf = ""
  table-id = 0
  metric = 100
  validate-nexthop = true
  community-list = []  # Match all routes
```

### Example 2: VRF-Aware Setup

```toml
# Import to VRF
[[vrfs]]
  [vrfs.config]
    name = "vrf-customer1"
    rd = "64512:100"

  [vrfs.netlink-import]
    enabled = true
    interfaces = ["eth1", "eth2"]

# Export from VRF
[[netlink.export.vrf-rules]]
  gobgp-vrf = "vrf-customer1"
  linux-vrf = "vrf-customer1"
  linux-table-id = 10
  metric = 50
  validate-nexthop = false
  community-list = ["64512:100"]
```

### Example 3: Selective Export with Communities

```toml
[[netlink.export.rules]]
  name = "customer-routes"
  vrf = ""
  table-id = 0
  metric = 100
  validate-nexthop = true
  community-list = ["65001:100", "65001:200"]  # Must have BOTH
  large-community-list = ["65001:1:100"]
```
```

#### 3.2 Update README.md

Add section:
```markdown
## Netlink Integration

GoBGP supports integration with the Linux routing table via netlink. This allows:
- Importing connected routes from Linux interfaces into GoBGP
- Exporting BGP routes to Linux routing tables
- Full VRF (Virtual Routing and Forwarding) support
- IPv4 and IPv6 address families

**Platform**: Linux only (requires kernel 4.3+ for VRF support)

See [Netlink Integration](docs/sources/netlink-integration.md) for details.
```

#### 3.3 Update docs/sources/configuration.md

Add reference to `[netlink]` configuration section with examples.

#### 3.4 CI/CD Documentation

Add note in `.github/workflows/` or CI documentation:
```markdown
## Platform-Specific Builds

GoBGP includes netlink integration which is **Linux-only**. This means:
- Linux builds (x86_64, arm64, arm) should succeed
- Non-Linux builds (Windows, macOS, FreeBSD) will fail compilation

This is expected behavior. The netlink feature requires the `vishvananda/netlink`
library which only supports Linux.
```

### Phase 4: Create purelb/gobgp-netlink Fork (1 hour)

#### Steps

1. **Create Fork on GitHub UI**
   - Navigate to https://github.com/acnodal/gobgp
   - Click "Fork" button
   - Select "purelb" organization
   - Repository name: `gobgp-netlink`
   - Description: "GoBGP with Linux netlink integration (production fork)"
   - Click "Create fork"

2. **Update Local Repository**
   ```bash
   cd /home/adamd/go/gobgp
   git remote add purelb https://github.com/purelb/gobgp-netlink.git
   git fetch purelb
   ```

3. **Create and Push main Branch**
   ```bash
   git checkout master
   git checkout -b main
   git push purelb main
   ```

4. **Set main as Default Branch**
   - Go to https://github.com/purelb/gobgp-netlink/settings/branches
   - Change default branch from `master` to `main`
   - Confirm the change

5. **Create Release Tag**
   ```bash
   git tag -a v1.0.0-netlink -m "Release v1.0.0-netlink

   First production release of GoBGP with netlink integration.
   Based on GoBGP v4.0.0.

   Features:
   - Linux routing table import/export
   - VRF-aware routing (IPv4 and IPv6)
   - Connected route import from interfaces
   - BGP route export to Linux routing tables
   - Community-based export filtering
   - Nexthop validation with ONLINK support
   - Comprehensive CLI tools
   - Statistics and observability

   Platform: Linux-only (kernel 4.3+ recommended)

   Documentation: docs/sources/netlink-integration.md

   🤖 Generated with Claude Code
   Co-Authored-By: Claude <noreply@anthropic.com>"

   git push purelb v1.0.0-netlink
   ```

6. **Create GitHub Release**
   - Go to https://github.com/purelb/gobgp-netlink/releases/new
   - Tag: `v1.0.0-netlink`
   - Title: "v1.0.0-netlink - First Release with Netlink Integration"
   - Description: Copy from tag message, add any additional notes
   - Check "Create a discussion for this release"
   - Click "Publish release"

7. **Update purelb/gobgp-netlink README**

   Add prominent notice at the top:
   ```markdown
   # GoBGP with Netlink Integration

   > **Note**: This is a production fork of GoBGP with Linux netlink integration.
   > Development happens at [acnodal/gobgp](https://github.com/acnodal/gobgp).

   ## Platform Support

   **Linux Only** - This fork includes netlink integration which requires Linux.
   - Supported: Linux (x86_64, arm64, arm)
   - Kernel: 4.3+ (for VRF support)
   - Not supported: Windows, macOS, FreeBSD

   ## Features

   All standard GoBGP features, plus:
   - Import connected routes from Linux interfaces to BGP
   - Export BGP routes to Linux routing tables
   - Full VRF (Virtual Routing and Forwarding) support
   - IPv4 and IPv6 address families
   - Community-based export filtering
   - Nexthop validation

   See [Netlink Integration](docs/sources/netlink-integration.md) for details.

   ## Releases

   This fork uses a modified version scheme: `vX.X.X-netlink`
   - v1.0.0-netlink: Based on GoBGP v4.0.0

   ## Upstream

   - **Upstream GoBGP**: [osrg/gobgp](https://github.com/osrg/gobgp)
   - **Development Fork**: [acnodal/gobgp](https://github.com/acnodal/gobgp)
   - **Production Fork**: [purelb/gobgp-netlink](https://github.com/purelb/gobgp-netlink) (this repository)
   ```

#### Verification Checklist
- [ ] purelb/gobgp-netlink repository created
- [ ] `main` branch is default branch
- [ ] v1.0.0-netlink tag pushed
- [ ] GitHub release created
- [ ] README updated with platform notice
- [ ] Fork relationship documented

## Platform Support Details

### Supported Platforms
- **Linux x86_64** (primary)
- **Linux arm64** (tested)
- **Linux arm** (tested)

### Minimum Requirements
- Kernel 4.3+ (for VRF support)
- Kernel 3.x (for basic netlink without VRF)
- CAP_NET_ADMIN capability (typically requires root)

### Unsupported Platforms
- Windows (netlink not available)
- macOS (netlink not available)
- FreeBSD (different network stack)
- Other Unix variants

### CI/CD Behavior

**Expected**: Linux builds succeed
**Expected**: Non-Linux builds fail with compilation errors

This is **correct behavior**. The vishvananda/netlink library only compiles on Linux.

**CI Configuration Note**:
```yaml
# Example CI configuration
jobs:
  build-linux:
    runs-on: ubuntu-latest
    # This should succeed

  build-windows:
    runs-on: windows-latest
    # This will fail - expected behavior
    continue-on-error: true  # Don't fail the workflow
```

## Maintenance Strategy

### acnodal/gobgp Repository
- **Purpose**: Development and testing
- **Branch**: `master` (contains netlink feature)
- **Upstream**: Tracks `osrg/gobgp` for updates
- **Workflow**:
  - Pull updates from upstream osrg/gobgp
  - Merge/rebase with netlink feature
  - Test thoroughly
  - Push to acnodal/gobgp master

### purelb/gobgp-netlink Repository
- **Purpose**: Production releases
- **Branch**: `main` (default)
- **Source**: Updates from acnodal/gobgp after testing
- **Workflow**:
  - Only update after comprehensive testing
  - Use version scheme: vX.X.X-netlink
  - Create GitHub releases with release notes
  - Stable release cadence

### Version Scheme

Format: `vX.X.X-netlink`

Examples:
- `v1.0.0-netlink`: First release (based on GoBGP v4.0.0)
- `v1.1.0-netlink`: Feature update
- `v1.0.1-netlink`: Bug fix
- `v2.0.0-netlink`: Major update (possibly based on newer GoBGP)

### Update Process

When GoBGP upstream releases new version (e.g., v4.1.0):

1. **Update acnodal/gobgp**:
   ```bash
   cd /home/adamd/go/gobgp
   git remote update upstream
   git checkout master
   git merge upstream/master
   # Resolve conflicts, test
   git push origin master
   ```

2. **Test thoroughly** (repeat Phase 2 testing)

3. **Update purelb/gobgp-netlink**:
   ```bash
   git checkout main
   git merge origin/master  # Merge from acnodal
   git tag -a v1.1.0-netlink -m "Release v1.1.0-netlink based on GoBGP v4.1.0"
   git push purelb main
   git push purelb v1.1.0-netlink
   ```

## Risk Assessment

### Low Risk Items
- Feature is Linux-specific and isolated from core BGP code
- No changes to BGP protocol handling
- Extensive error handling and validation
- Statistics and observability built-in
- Comprehensive testing planned

### Medium Risk Items
- **Non-Linux CI builds will fail**: This is expected and documented
- **Requires elevated permissions**: Root or CAP_NET_ADMIN needed
- **Direct kernel interaction**: Via netlink library (well-tested)
- **VPN NLRI handling**: Complex type switching for IPv4/IPv6

### Mitigation Strategies

1. **Platform Compatibility**:
   - Document Linux-only requirement prominently in README
   - Add CI notes about expected build failures
   - Provide clear error messages on unsupported platforms

2. **Permissions**:
   - Document permission requirements in installation guide
   - Provide clear error messages for permission issues
   - Show how to use capabilities instead of root

3. **Testing**:
   - Comprehensive Phase 2 testing (2-3 days)
   - Long-running stability tests
   - Scale testing with 1000+ routes
   - Error injection testing

4. **Observability**:
   - Detailed logging at debug level
   - Statistics for import/export operations
   - CLI commands to inspect state
   - Error tracking with timestamps

## Success Criteria

All criteria must be met before Phase 4 (fork creation):

- [ ] Netlink branch merged to acnodal/gobgp master (Phase 1)
- [ ] All Phase 2 test categories completed with >95% pass rate
- [ ] No critical bugs or regressions identified
- [ ] Documentation complete and reviewed (Phase 3)
- [ ] CI builds succeed on Linux
- [ ] Non-Linux CI failures documented as expected behavior
- [ ] Performance metrics acceptable (convergence time, memory, CPU)
- [ ] Code reviewed and approved
- [ ] purelb/gobgp-netlink fork created with main branch (Phase 4)
- [ ] v1.0.0-netlink release tagged and published
- [ ] Release notes and documentation published

## Rollback Plan

### If Critical Issues Found Before Release

1. **Stop the process** - Do not create purelb/gobgp-netlink fork
2. **Document the issue** in GitHub issue tracker
3. **Fix the issue** in netlink branch
4. **Re-test** (Phase 2)
5. **Resume from Phase 3** once fixed

### If Critical Issues Found After Release

1. **Document issue** in GitHub with severity label
2. **Create hotfix branch** from purelb/gobgp-netlink main
3. **Fix and test** the issue
4. **Release v1.0.1-netlink** with fix
5. **Update documentation** with known issues and workarounds

### If Merge Causes Regressions

1. **Revert merge** in acnodal/gobgp
   ```bash
   git revert -m 1 <merge-commit-hash>
   ```
2. **Analyze root cause** of regression
3. **Fix in netlink branch**
4. **Re-test thoroughly**
5. **Retry merge**

## Timeline and Milestones

### Day 1 (4-8 hours)
- [ ] Phase 1: Merge to acnodal/gobgp master (4 hours)
- [ ] Begin Phase 2: Import testing (4 hours)

### Day 2 (8 hours)
- [ ] Phase 2: Complete import testing
- [ ] Phase 2: Begin export testing (4 hours)

### Day 3 (8 hours)
- [ ] Phase 2: Complete export testing
- [ ] Phase 2: Combined import/export testing
- [ ] Phase 2: Begin scale testing

### Day 4 (8 hours)
- [ ] Phase 2: Complete scale testing
- [ ] Phase 2: Begin stability testing (long-running)

### Day 5 (8 hours)
- [ ] Phase 2: Complete stability testing
- [ ] Phase 2: CLI testing
- [ ] Phase 3: Documentation updates (2 hours)

### Day 6 (4-6 hours)
- [ ] Phase 4: Create purelb/gobgp-netlink fork (1 hour)
- [ ] Final verification
- [ ] Publish release
- [ ] Buffer time for any issues

**Total Estimated Time**: 5-6 days

## Questions and Issues

Track all questions, issues, and decisions in:
- GitHub Issue: https://github.com/acnodal/gobgp/issues/2
- Local tracking: This document

## References

### Key Source Files
- `pkg/server/netlink_export.go` - Export engine
- `pkg/server/znetlink.go` - Import engine
- `pkg/netlink/netlink.go` - Netlink client wrapper
- `cmd/gobgp/netlink.go` - CLI commands
- `pkg/config/oc/types.go` - Configuration types
- `proto/api/gobgp.proto` - gRPC API definitions

### Key Commits
- VRF export fix: Separated VPN and unicast path processing
- VRF import fix: Changed to iterate globalRib.Vrfs
- IPv6 VPN export: Added LabeledVPNIPv6AddrPrefix handling
- IPv6 withdrawal fix: Extract IP prefix from VPN NLRI
- ONLINK support: Added VRF device LinkIndex for unreachable nexthops

### Documentation
- `docs/sources/netlink-integration.md` - Main netlink documentation (to be created)
- `docs/netlink-export-design.md` - Original design document
- `tools/README.md` - Protocol buffer generation

## Change Log

**2025-11-19**: Initial plan created
- 5-6 day timeline established
- Linux-only deployment confirmed
- Version scheme: v1.0.0-netlink
- Main branch for purelb/gobgp-netlink
- CI crossbuild failures accepted as expected

---

**Status**: Ready for execution
**Next Step**: Begin Phase 1 (Merge to acnodal/gobgp master)

For questions or updates, see: https://github.com/acnodal/gobgp/issues/2
