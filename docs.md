# clabernetes-launch Documentation

This document provides comprehensive documentation for the clabernetes-launch fork, focusing on the changes made from the upstream srl-labs/clabernetes repository.

## Overview

clabernetes-launch is a fork of the clabernetes project that adds enhanced terminal access capabilities through TTYD and tmux integration. The fork maintains all core functionality of clabernetes (a Kubernetes controller that deploys ContainerLab topologies into a Kubernetes cluster) while adding web-based terminal access to nodes.

## Architecture

### Core Components

1. **Launcher** (`launcher/clabernetes.go`): 
   - Responsible for starting ContainerLab nodes in containers
   - Manages container lifecycle, networking, and monitoring
   - Added TTYD/tmux integration for web-based terminal access

2. **Controller** (`controllers/`): 
   - Kubernetes controller that watches for Topology custom resources
   - Deploys and manages ContainerLab topologies in the cluster

3. **Build System**:
   - Custom Dockerfile (`build/launcher.Dockerfile`) for building the launcher image
   - GitHub Actions workflows for CI/CD

### Data Flow

1. User applies a ContainerLab topology as a Kubernetes Custom Resource
2. Controller detects the resource and launches pods running the launcher
3. Launcher starts ContainerLab inside each pod, creating the network topology
4. Launcher monitors container health and provides status probes
5. With TTYD enabled, users can access node terminals via web browser

## API

clabernetes-launch extends the Kubernetes API with Custom Resource Definitions (CRDs) for managing ContainerLab topologies. The fork does not modify the core API but enhances the launcher component.

Key CRDs:
- `Topology`: Defines a ContainerLab network topology
- `Node`: Represents a node in the topology (managed by the launcher)

## Deployment

### Prerequisites

- Kubernetes cluster (v1.18+)
- kubectl configured to access the cluster
- ContainerLab installed (handled by the launcher)

### Installation

1. Build the launcher image:
   ```bash
   make build-manager
   ```

2. Deploy the controller and launcher:
   ```bash
   make deploy
   ```

3. Verify deployment:
   ```bash
   kubectl get pods -n clabernetes-system
   ```

### Configuration

The launcher supports additional environment variables for TTYD functionality:

| Variable | Description | Default |
|----------|-------------|---------|
| `TTYD_SHELL` | Shell to run in TTYD terminal (set to "attach" to attach to container's main process) | empty (disabled) |
| `LauncherTCPProbePort` | TCP port for status probes | 0 (disabled) |
| `LauncherSSHProbePort` | SSH port for status probes | 22 |
| `LauncherSSHProbeUsername` | Username for SSH probes | empty |
| `LauncherSSHProbePassword` | Password for SSH probes | empty |

## Changes from Upstream (srl-labs/clabernetes)

This section details the specific changes made in this fork compared to the upstream repository.

### 1. TTYD/Tmux Integration (Major Feature)

**Files Modified:**
- `launcher/clabernetes.go`
- `build/launcher.Dockerfile`
- `build/launcher/.tmux.conf` (new)

**Changes:**

#### launcher/clabernetes.go
- Added `startTTYD()` method that:
  - Checks for `TTYD_SHELL` environment variable
  - Waits for the node container to be ready
  - Starts TTYD web terminal server on port 7681
  - Integrates with tmux for session persistence
  - Supports both shell execution and container attachment modes
- Added constants:
  - `ttydWaitInterval = 5 * time.Second`
  - `ttydPort = "7681"`
  - `tmuxSessionName = "clabernetes"`
- Enhanced logging for TTYD operations
- Added graceful shutdown handling for TTYD processes

#### build/launcher.Dockerfile
- Added `ARG TTYD_VERSION="1.7.7"`
- Added installation of TTYD: `curl -L https://github.com/tsl0922/ttyd/releases/download/${TTYD_VERSION}/ttyd.x86_64 -o /usr/local/bin/ttyd && chmod +x /usr/local/bin/ttyd`
- Added tmux to dependencies: `inetutils-ping`, `traceroute`, `tmux`
- Copied tmux configuration: `COPY build/launcher/.tmux.conf /root/.tmux.conf`
- Comment: "# WEB access tmux config without key-binds"

#### build/launcher/.tmux.conf (new file)
- Contains tmux configuration optimized for web access via TTYD
- Disables conflicting key bindings
- Sets up proper terminal behavior for web-based interaction

### 2. Build and CI/CD Updates

**Files Modified:**
- `.github/workflows/*.yaml` (multiple files)
- `.develop/devspace.yaml`

**Changes:**
- Updated workflow files for building and releasing the fork
- Modified devspace.yaml for development environment configuration
- Adjusted Docker build arguments and paths
- Updated Makefile targets where necessary

### 3. Constants and Utility Updates

**Files Modified:**
- `constants/common.go`

**Changes:**
- Added new constants for TTYD functionality (referenced in launcher)
- Maintained backward compatibility with existing constants

### 4. Controller Enhancements

**Files Modified:**
- `controllers/topology/configmap.go`
- `controllers/topology/deployment.go`
- `controllers/topology/servicefabric.go`

**Changes:**
- Enhanced topology rendering capabilities
- Improved service fabric integration
- Added support for new rendering templates
- Fixed various bugs in topology deployment logic

### 5. Test and Example Updates

**Files Modified:**
- `test/e2e/golden/*.yaml`
- Various files in `controllers/topology/render-service/`
- Examples in `examples/` directory

**Changes:**
- Updated golden test files to reflect new behavior
- Added/updated render service templates for different vendors
- Enhanced example topologies to demonstrate new features

### 6. Other Miscellaneous Changes

**Files Modified:**
- `util/containerlab/types.go`
- Various documentation and configuration files

**Changes:**
- Minor utility function updates
- Documentation improvements
- Dependency version updates

## Usage with TTYD

To use the TTYD functionality:

1. Set the `TTYD_SHELL` environment variable when deploying the launcher:
   ```bash
   # Example: Start a bash shell in the terminal
   export TTYD_SHELL="bash"
   
   # Example: Attach to the container's main process
   export TTYD_SHELL="attach"
   ```

2. Access the web terminal:
   - Find the node's IP address or hostname
   - Open a browser to `http://<node-address>:7681`
   - You should see a terminal interface connected to the node

3. TMUX Integration:
   - The terminal automatically starts inside a tmux session named "clabernetes"
   - This allows for session persistence and multiple panes if needed
   - The tmux configuration is optimized for web access (no conflicting key bindings)

## Development

### Building

```bash
# Build the manager binary
make build-manager

# Build the launcher Docker image
make docker-build-manager
```

### Testing

```bash
# Run unit tests
make test

# Run end-to-end tests
make test-e2e
```

### Documentation

The upstream documentation at https://containerlab.dev/manual/clabernetes still applies, with the addition of TTYD/tmux features documented above.

## License

This project is licensed under the Apache License 2.0 - see the LICENSE file for details.

## Acknowledgments

- Original clabernetes project by srl-labs
- TTYD project for web terminal sharing
- tmux developers for terminal multiplexing