# Doo

Doo is a local web tool for LLM API pilot studies: trying prompts, models, and request settings before running a larger evaluation.

<p align="center"><em>My three-year-old daughter loves Baby Shark Dance. The “Doo-doo” in the song gave this project its name.</em></p>

## Why Doo?

Doo is built for pilot studies: small, exploratory experiments before a full benchmark or evaluation. I want to try a few representative inputs, compare models and parameter settings, and check API behavior before committing to a larger run.

[Cherry Studio](https://www.cherryai.com/docs/en/cherrystudio/preview/) is aimed at using LLMs for real work. For my own work, I prefer tools such as Codex and Claude Code. I built Doo for the earlier stage, when I'm still figuring out which model to use, how to call it, and what to measure.

In Doo, I can send the same input to several model presets, inspect requests and responses, and compare latency and token usage alongside the answers. Once a configuration works, I can export the request as curl or Python and take it into an experiment script. The goal is to catch problems and narrow down choices while the experiment is still small.

## Quick Start

### Download a Release

The easiest way to get started is to download a prebuilt binary from the [latest release](https://github.com/ChenZhongPu/Doo/releases/latest). Choose the archive for your operating system and processor.

Extract the archive and open a terminal in the extracted folder. On macOS or Linux, run:

```bash
./doo
```

On Windows, run from PowerShell:

```powershell
.\doo.exe
```

Doo opens your browser at [localhost:8787](http://localhost:8787). Add a provider and a model preset under **Config**, then select models and send a prompt under **Compare**. Settings and run history are stored in `~/.doo` by default.

### Build from Source

To build it yourself, install Go 1.26 or later, Node.js 18 or later, npm, and Make. From the repository root:

```bash
npm --prefix web ci
make all
./bin/doo
```

The executable is written to `bin/doo`.

### Command-line Options

For example, to use another port and data directory:

```bash
./doo --addr 127.0.0.1:9000 --data ./doo-data
```

| Flag | Default | Description |
|---|---|---|
| `--addr` | Unset | Override `--host` and `--port` with a listen address (`host:port`) |
| `--host` | `localhost` | Server listen host |
| `--port` | `8787` | Server listen port |
| `--data` | `~/.doo` | Data directory storing `doo.db` and `master.key` |
| `--max-targets` | `8` | Maximum concurrent targets in comparison (`0` for unlimited) |
| `--open` | `true` | Automatically open default browser on launch |
| `--version` | `false` | Print version information and exit |
