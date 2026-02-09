# Project Bootstrapping

Make a new directory to hold the files of your `Microbus`-based project.

```shell
mkdir myproject
cd myproject
```

Instruct your coding agent to:

> HEY CLAUDE...
>
> Follow the workflow at https://raw.githubusercontent.com/microbus-io/fabric/refs/heads/main/setup/bootstrap.md to bootstrap Microbus.

Your project structure will now look like this:

```
myproject/
├── .claude/                    # Claude Code setup
│   ├── rules/                  # Instructions for coding agents
│   │   └── microbus.md
│   └── skills/                 # Claude Code skills
├── .vscode/
│   └── launch.json             # VSCode launch file
├── main/
│   ├── env.yaml                # Environment variables for main app
│   └── main.go                 # Main application
├── .gitignore
├── AGENTS.md                   # Instructions for coding agents
├── CLAUDE.md                   # Instructions for Claude
├── config.yaml                 # Configuration properties
├── config.local.yaml           # git ignored configuration properties
├── env.yaml                    # Environment variables
├── env.local.yaml              # git ignored environment variables
├── go.mod
└── go.sum
```
