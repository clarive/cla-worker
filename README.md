# Clarive Worker

The Clarive Worker (`cla-worker`) is a pull-based agent used for
sending/retrieving files and running commands on remote servers as part of
[Clarive](https://clarive.com) DevOps automation.

This is the official Go implementation, replacing the
[previous Node.js version](https://github.com/clarive/cla-worker) which is now
deprecated. The Go version produces a single static binary with no runtime
dependencies, lower memory footprint, faster startup and built-in
cross-platform support.

## How It Works

The worker is an executable that runs on each server in your infrastructure
where you need to perform DevOps operations such as executing shell commands,
sending files (ship) and retrieving files (fetch).

It is a _pull_ agent, meaning it initiates an outbound SSE (Server-Sent Events)
connection to the Clarive server and waits for instructions. This makes it
ideal for servers behind firewalls that are not directly reachable via SSH or a
push agent.

## Capabilities

A running worker can:

- Execute arbitrary shell commands on the host
- Receive a file from the Clarive server and write it locally
- Send a local file to the Clarive server
- Evaluate JavaScript expressions in a sandboxed engine
- Report its tags so the server can route jobs by capability

## Supported Platforms

Pre-built binaries are available for:

| OS | Architecture |
|---|---|
| Linux | amd64, arm64 |
| macOS | amd64 (Intel), arm64 (Apple Silicon) |
| Windows | amd64 |

## Installation

Download the latest release from the
[Releases](https://github.com/clarive/cla-worker-go/releases) page and place
the binary anywhere in your `PATH`.

The worker is a single static binary with no prerequisites. The only requirement
is that the host **can reach the Clarive server** over HTTP/HTTPS.

## Quick Start

```bash
# 1. Register the worker with a project passkey
cla-worker register --url https://your-clarive-server --passkey YOUR_PASSKEY --save

# 2. Start the worker
cla-worker run
```

## Configuration

Configuration is loaded from either YAML or TOML files, auto-detected by file
extension. The search order is:

1. `--config` flag (explicit path)
2. `CLA_WORKER_CONFIG` environment variable
3. `./cla-worker.yml` or `./cla-worker.toml` (current directory)
4. `~/cla-worker.yml` or `~/cla-worker.toml` (home directory)
5. `/etc/cla-worker.yml` or `/etc/cla-worker.toml`

### YAML example (`cla-worker.yml`)

```yaml
id: myworker
token: 97d317df5ad3fbb68334657ec94aefe6
url: https://your-clarive-server
tags:
  - java
  - nodejs
```

### TOML example (`cla-worker.toml`)

```toml
id = "myworker"
token = "97d317df5ad3fbb68334657ec94aefe6"
url = "https://your-clarive-server"
tags = ["java", "nodejs"]
```

### Configuration fields

| Field | Description |
|---|---|
| `id` | Unique worker identifier |
| `token` | Authentication token from registration |
| `url` | Clarive server URL |
| `passkey` | Project passkey (used during registration) |
| `tags` | Comma-separated string or list of capability tags |
| `origin` | Origin identifier (defaults to `user@host/pid`) |
| `verbose` | Verbosity level (integer) |
| `logfile` | Path to log file (daemon mode) |
| `pidfile` | Path to PID file (daemon mode) |
| `chunk_size` | File transfer chunk size in bytes (default: 65536) |
| `registrations` | List of `{id, token}` pairs for multi-registration setups |

## Commands

### Registering

Register the worker with a project passkey obtained from the Clarive UI under
**Deploy > Workers**:

```bash
cla-worker register --url https://your-clarive-server --passkey YOUR_PASSKEY
```

The server returns an ID-token pair unique to this worker instance. To save the
credentials directly to the config file:

```bash
cla-worker register --passkey YOUR_PASSKEY --save
```

You can assign a custom ID:

```bash
cla-worker register --id myworker --passkey YOUR_PASSKEY --save
```

> **Security note:** The ID-token pair is analogous to a username/password. Keep
> it safe. If compromised, an attacker could impersonate the worker.

### Unregistering

```bash
cla-worker unregister --id myworker --token YOUR_TOKEN
```

### Running (foreground)

```bash
cla-worker run --id myworker --token YOUR_TOKEN
```

Or if you have a config file with a single registration:

```bash
cla-worker run
```

With multiple registrations in the config file, specify which one:

```bash
cla-worker run --id myworker
```

### Daemon mode

Start in the background:

```bash
cla-worker start --id myworker
```

Check status:

```bash
cla-worker status
```

Stop the daemon:

```bash
cla-worker stop
```

### OS service

Install as a system service (systemd, launchd, Windows Service):

```bash
cla-worker install
```

Remove the service:

```bash
cla-worker remove
```

### File operations

Push a local file to the server:

```bash
cla-worker push --file /path/to/file --key my-file-key
```

Download a file from the server:

```bash
cla-worker pop --key my-file-key --file /path/to/output
```

## Worker Tags

Tags identify a worker's capabilities. When writing Clarive rulebooks, you can
route jobs to any worker that has the required tags:

```bash
cla-worker run --tags java,nodejs
```

Then in a rulebook:

```yaml
do:
  shell:
    worker: { tags: ['java'] }
    cmd: javac MyClass.java
```

## Worker Security

The worker runs as the OS user that launched it. Commands sent by the server
execute with that user's permissions. Be mindful of `sudo` configurations or
other privilege escalation mechanisms on the host.

## Building from Source

Requires Go 1.24+.

```bash
make build          # Build binary to bin/cla-worker
make test           # Run unit tests with race detector
make test-integration  # Run integration tests
make cross          # Cross-compile for all platforms
make cover          # Generate coverage report
```

## License

Copyright Clarive Software. All rights reserved.
