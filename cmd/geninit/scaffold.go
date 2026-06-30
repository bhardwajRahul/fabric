/*
Copyright (c) 2023-2026 Microbus LLC and various contributors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const claudeMarker = "**CRITICAL**: This project uses the Microbus framework. " +
	"Read all `.md` files in `.claude/rules/` before starting any task."

// generator scaffolds a Microbus project rooted at dir.
type generator struct {
	root string
}

func (g *generator) run() error {
	steps := []func() error{
		g.mainPackage,
		g.agentFiles,
		g.configFiles,
		g.envFiles,
		g.gitIgnore,
		g.vscodeLaunch,
		g.claudeSettings,
		g.signingKey,
	}
	for _, step := range steps {
		err := step()
		if err != nil {
			return err
		}
	}
	return nil
}

// mainPackage writes main/main.go and main/env.yaml.
func (g *generator) mainPackage() error {
	err := g.create("main/main.go", template("main.go.txt"))
	if err != nil {
		return err
	}
	return g.prependIfMissing("main/env.yaml", "MICROBUS_DEPLOYMENT: LOCAL\n", "MICROBUS_DEPLOYMENT")
}

// agentFiles writes the root CLAUDE.md.
func (g *generator) agentFiles() error {
	return g.prependIfMissing("CLAUDE.md", claudeMarker+"\n", "Read all `.md` files in `.claude/rules/`")
}

// configFiles writes config.yaml and config.local.yaml.
func (g *generator) configFiles() error {
	err := g.create("config.yaml", template("config.yaml.txt"))
	if err != nil {
		return err
	}
	return g.create("config.local.yaml", template("config.local.yaml.txt"))
}

// envFiles writes env.yaml and env.local.yaml.
func (g *generator) envFiles() error {
	err := g.create("env.yaml", template("env.yaml.txt"))
	if err != nil {
		return err
	}
	return g.create("env.local.yaml", template("env.yaml.txt"))
}

// gitIgnore appends the Microbus ignore block.
func (g *generator) gitIgnore() error {
	block := "# Microbus\n*.local.*\n/main/main\n/main/__debug_bin*\n.DS_Store\n"
	return g.appendIfMissing(".gitignore", block, "# Microbus")
}

// vscodeLaunch writes .vscode/launch.json with a Main launch configuration.
func (g *generator) vscodeLaunch() error {
	path := ".vscode/launch.json"
	if !g.exists(path) {
		return g.create(path, template("launch.json.txt"))
	}
	body, err := os.ReadFile(g.path(path))
	if err != nil {
		return err
	}
	if strings.Contains(string(body), `"Main"`) {
		g.skip(path)
		return nil
	}
	g.warn("%s exists without a Main configuration; add it manually:\n%s", path, template("launch.json.txt"))
	return nil
}

// claudeSettings writes .claude/settings.json, merging permissions into an existing file.
func (g *generator) claudeSettings() error {
	path := ".claude/settings.json"
	if !g.exists(path) {
		return g.create(path, template("settings.json.txt"))
	}
	merged, changed, err := mergeSettings(g.read(path), template("settings.json.txt"))
	if err != nil {
		g.warn("%s could not be parsed for merge (%v); reconcile its permissions.allow manually", path, err)
		return nil
	}
	if !changed {
		g.skip(path)
		return nil
	}
	return g.write(path, merged, "merged permissions into")
}

// mergeSettings adds any allow-rules from src that are absent from dst, preserving dst's order.
func mergeSettings(dst, src string) (string, bool, error) {
	var dstDoc, srcDoc map[string]any
	err := json.Unmarshal([]byte(dst), &dstDoc)
	if err != nil {
		return "", false, err
	}
	err = json.Unmarshal([]byte(src), &srcDoc)
	if err != nil {
		return "", false, err
	}
	dstAllow := allowList(dstDoc)
	have := map[string]bool{}
	for _, r := range dstAllow {
		have[r] = true
	}
	changed := false
	for _, r := range allowList(srcDoc) {
		if !have[r] {
			dstAllow = append(dstAllow, r)
			have[r] = true
			changed = true
		}
	}
	if !changed {
		return dst, false, nil
	}
	perms, _ := dstDoc["permissions"].(map[string]any)
	if perms == nil {
		perms = map[string]any{}
		dstDoc["permissions"] = perms
	}
	perms["allow"] = dstAllow
	out, err := json.MarshalIndent(dstDoc, "", "  ")
	if err != nil {
		return "", false, err
	}
	return string(out) + "\n", true, nil
}

// allowList extracts permissions.allow as a slice of strings.
func allowList(doc map[string]any) []string {
	perms, _ := doc["permissions"].(map[string]any)
	raw, _ := perms["allow"].([]any)
	out := make([]string, 0, len(raw))
	for _, v := range raw {
		s, ok := v.(string)
		if ok {
			out = append(out, s)
		}
	}
	return out
}

// create writes content to path only if the file does not already exist.
func (g *generator) create(rel, content string) error {
	if g.exists(rel) {
		g.skip(rel)
		return nil
	}
	return g.write(rel, content, "created")
}

// prependIfMissing creates the file with content, or prepends content when marker is absent.
func (g *generator) prependIfMissing(rel, content, marker string) error {
	if !g.exists(rel) {
		return g.write(rel, content, "created")
	}
	existing := g.read(rel)
	if strings.Contains(existing, marker) {
		g.skip(rel)
		return nil
	}
	return g.write(rel, content+existing, "prepended to")
}

// appendIfMissing creates the file with content, or appends content when marker is absent.
func (g *generator) appendIfMissing(rel, content, marker string) error {
	if !g.exists(rel) {
		return g.write(rel, content, "created")
	}
	existing := g.read(rel)
	if strings.Contains(existing, marker) {
		g.skip(rel)
		return nil
	}
	sep := "\n"
	if strings.HasSuffix(existing, "\n") {
		sep = ""
	}
	return g.write(rel, existing+sep+content, "appended to")
}

func (g *generator) path(rel string) string {
	return filepath.Join(g.root, filepath.FromSlash(rel))
}

func (g *generator) exists(rel string) bool {
	_, err := os.Stat(g.path(rel))
	return err == nil
}

func (g *generator) read(rel string) string {
	b, err := os.ReadFile(g.path(rel))
	if err != nil {
		return ""
	}
	return string(b)
}

func (g *generator) write(rel, content, verb string) error {
	full := g.path(rel)
	err := os.MkdirAll(filepath.Dir(full), 0755)
	if err != nil {
		return err
	}
	err = os.WriteFile(full, []byte(content), 0644)
	if err != nil {
		return err
	}
	fmt.Printf("  %s %s\n", verb, rel)
	return nil
}

func (g *generator) skip(rel string) {
	fmt.Printf("  exists, left intact: %s\n", rel)
}

func (g *generator) warn(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "  warning: "+format+"\n", args...)
}
