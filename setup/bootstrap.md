---
name: Bootstrapping a Project to use Microbus
description: Bootstraps a project to use the Microbus framework.
---

## Workflow

Copy this checklist and track your progress:

```
Bootstrap a project to use Microbus:
- [ ] Step 1: Confirm Director
- [ ] Step 2: Init the Module
- [ ] Step 3: Download agent rules and skills
- [ ] Step 4: Init the project
```

#### Step 1: Confirm Directory

Confirm with the user that they would like to bootstrap `Microbus` in the current directory. If they indicate another directory, create it using `mkdir -p alternate_directory` and change to it. All next steps should be done under this directory.

#### Step 2: Init the Module

If the current directory includes a `go.mod` file, skip this step.

Ask the user for the name of the module to set for this project. Then use it as the input to `go mod init`.

```shell
go mod init github.com/mycompany/myproject
```

#### Step 3: Download Agent Rules and Skills

Download the latest coding agent rules and skills from Github.

```shell
git clone --depth 1 https://github.com/microbus-io/fabric tmp/microbus-fabric
rm -rf .claude/rules/microbus.md
rm -rf .claude/rules/skills/microbus
cp -r tmp/microbus-fabric/.claude .
rm -rf tmp/microbus-fabric
```

The `.claude` directory should include the following.

```
.claude/
├── rules/
│   └── microbus.md
└── skills/
    └── microbus/
```

#### Step 4: Init the Project

Follow the skill in `.claude/skills/microbus/init-project` to initialize the project.
