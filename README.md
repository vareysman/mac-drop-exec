# mac-drop-exec

> https://github.com/vareysman/mac-drop-exec

A lightweight macOS LaunchDaemon written in Go that executes shell commands as root via file-based IPC.

Drop a shell command into `/tmp/xipt` — the daemon picks it up, runs it with root privileges, then removes the file. If the command fails, the error output is written to `/tmp/xopt`.

## How it works

```
/tmp/xipt  →  [mac-drop-exec]  →  /bin/sh -c <command>
                                         │
                                    on failure
                                         ↓
                                    /tmp/xopt
```

1. Daemon polls `/tmp/xipt` every **10 seconds**
2. If the file exists — reads the command, executes it via `/bin/sh -c`
3. Removes `/tmp/xipt` after execution
4. On failure — writes combined error + output to `/tmp/xopt`

Commands time out after **5 minutes**. Maximum command size is **256 KB**.

## Requirements

- macOS (tested on macOS 12+)
- Go 1.26+
- Root privileges for installation

## Installation

```sh
sudo ./install.sh install
```

The script will:
- Build the binary with `go build`
- Install it to `/usr/local/sbin/admin-daemon`
- Register and start the LaunchDaemon (`com.admin.daemon`)

Logs are written to:
- `/var/log/admin-daemon.log` — stdout
- `/var/log/admin-daemon.err` — stderr

## Usage

```sh
# Run a command as root
echo "whoami > /tmp/result.txt" > /tmp/xipt

# Wait up to 10 seconds, then read the result
cat /tmp/result.txt

# On failure, check the error output
cat /tmp/xopt
```

## Configuration

The input and output paths can be overridden via environment variables in the plist:

| Variable           | Default      | Description              |
|--------------------|--------------|--------------------------|
| `ADMIN_DAEMON_IN`  | `/tmp/xipt`  | Path polled for commands |
| `ADMIN_DAEMON_OUT` | `/tmp/xopt`  | Path for error output    |

## Managing the daemon

```sh
# Check status
sudo ./install.sh status

# Uninstall
sudo ./install.sh uninstall
```

## Security considerations

- The daemon runs as **root** — only trusted local users should be able to write to the input path
- Restrict write access to `/tmp/xipt` as appropriate for your environment
- Commands are executed directly via `/bin/sh -c` with no sandboxing

## License

MIT
