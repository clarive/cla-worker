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
| Linux | x64, arm64 |
| macOS | x64 (Intel), arm64 (Apple Silicon) |
| Windows | x64, arm64 |

## Installation

Download the binary for your platform from the
[latest release](https://github.com/clarive/cla-worker/releases/latest):

| Platform | Download |
|---|---|
| Linux x64 | [cla-worker-linux-x64.tar.gz](https://github.com/clarive/cla-worker/releases/latest/download/cla-worker-linux-x64.tar.gz) |
| Linux arm64 | [cla-worker-linux-arm64.tar.gz](https://github.com/clarive/cla-worker/releases/latest/download/cla-worker-linux-arm64.tar.gz) |
| macOS x64 (Intel) | [cla-worker-darwin-x64.tar.gz](https://github.com/clarive/cla-worker/releases/latest/download/cla-worker-darwin-x64.tar.gz) |
| macOS arm64 (Apple Silicon) | [cla-worker-darwin-arm64.tar.gz](https://github.com/clarive/cla-worker/releases/latest/download/cla-worker-darwin-arm64.tar.gz) |
| Windows x64 | [cla-worker-windows-x64.zip](https://github.com/clarive/cla-worker/releases/latest/download/cla-worker-windows-x64.zip) |
| Windows arm64 | [cla-worker-windows-arm64.zip](https://github.com/clarive/cla-worker/releases/latest/download/cla-worker-windows-arm64.zip) |

Extract the archive and place the `cla-worker` binary anywhere in your `PATH`.
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
| `verbose` | Verbosity level: 0 = INFO (default), 1+ = DEBUG |
| `logfile` | Path to log file (daemon mode) |
| `pidfile` | Path to PID file (daemon mode) |
| `chunk_size` | File transfer chunk size in bytes (default: 65536) |
| `registrations` | List of `{id, token, url}` entries for multi-registration setups |

### Worker ID restrictions

The worker ID is used in URL query parameters, file paths (pidfile, logfile)
and as a service identifier. Avoid the following characters:

- **Spaces** — break command-line arguments and URL parameters
- **Slashes** (`/` and `\`) — break file paths used for pidfiles and logfiles
- **URL-reserved characters** (`?`, `&`, `#`, `%`) — break server communication
- **Quotes** (`"`, `'`, `` ` ``) — break shell commands and config file parsing
- **Newlines, tabs or other control characters** — break config parsing

Safe characters: letters, digits, hyphens (`-`), underscores (`_`), dots (`.`)
and the at sign (`@`). The default auto-generated format `user@hostname`
(e.g. `joe@server1`) uses only safe characters.

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

Install as a system service (systemd on Linux, launchd on macOS, Windows
Service on Windows):

```bash
cla-worker install -c /path/to/cla-worker.toml
```

Start/stop the installed service:

```bash
cla-worker start
cla-worker stop
```

Remove the service:

```bash
cla-worker remove
```

See [Linux systemd service](#linux-systemd-service) and
[Windows service](#windows-service) below for detailed platform guides.

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

## Debug Mode

By default the worker logs at INFO level. Periodic heartbeat events
(`worker.connect`) are logged once at startup and then suppressed to avoid
noise. To see all messages, including heartbeats and internal diagnostics,
enable debug mode using the `-v` flag or the `verbose` config field.

Using the command-line flag:

```bash
cla-worker run -v
```

Or in the config file (`cla-worker.yml`):

```yaml
verbose: 1
```

Debug mode is also supported in daemon mode:

```bash
cla-worker run --daemon -v
```

## Linux systemd service

On Linux, `cla-worker install` creates a systemd unit file so the worker
starts on boot.

```bash
# Place binary and register
sudo mkdir -p /opt/cla-worker
sudo cp cla-worker /opt/cla-worker/
sudo chmod +x /opt/cla-worker/cla-worker
cd /opt/cla-worker
sudo ./cla-worker register --url https://clarive.example.com --passkey YOUR_PASSKEY --save

# Install and start the service
sudo ./cla-worker install -c /opt/cla-worker/cla-worker.toml
sudo systemctl enable cla-worker
sudo systemctl start cla-worker
```

Manage with standard systemctl commands:

```bash
sudo systemctl status cla-worker     # check status
sudo journalctl -u cla-worker -f     # view logs
sudo systemctl restart cla-worker    # restart after config changes
sudo systemctl stop cla-worker       # stop
```

To run under a dedicated user, create a system user and update the unit file:

```bash
sudo useradd -r -s /bin/false cla-worker
sudo chown -R cla-worker:cla-worker /opt/cla-worker
```

Add `User=cla-worker` and `Group=cla-worker` under `[Service]` in
`/etc/systemd/system/cla-worker.service`, then:

```bash
sudo systemctl daemon-reload
sudo systemctl restart cla-worker
```

Remove the service:

```bash
sudo systemctl stop cla-worker
sudo systemctl disable cla-worker
sudo /opt/cla-worker/cla-worker remove
```

## Windows service

On Windows, `cla-worker install` registers the worker as a Windows Service.
Run all commands from an **Administrator** prompt.

```powershell
# Place binary and register
mkdir C:\cla-worker
copy cla-worker.exe C:\cla-worker\
cd C:\cla-worker
.\cla-worker.exe register --url https://clarive.example.com --passkey YOUR_PASSKEY --save

# Install and start the service
.\cla-worker.exe install -c C:\cla-worker\cla-worker.toml
.\cla-worker.exe start
```

The service is named **cla-worker** (display name **Clarive Worker**) and
starts automatically on boot.

Manage with `sc` or the Services GUI (`services.msc`):

```powershell
sc query cla-worker                            # check status
sc stop cla-worker                             # stop
sc stop cla-worker && sc start cla-worker      # restart
```

View logs:

```powershell
Get-Content C:\cla-worker\cla-worker.log -Tail 50 -Wait
```

To run under a specific user instead of Local System:

```powershell
sc config cla-worker obj= "DOMAIN\username" password= "password"
sc stop cla-worker
sc start cla-worker
```

Remove the service:

```powershell
sc stop cla-worker
C:\cla-worker\cla-worker.exe remove
```

If the service fails to start, check the log file, verify the config path,
and look for errors in the Windows Event Viewer (Application log).

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
