// Split *api/client.go into client.go + endpoints.go per upgrade-v1-27-1.
// Run with: go run split_client.go path/to/myservice/myserviceapi/client.go
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

func main() {
	args := os.Args[1:]
	// Strip a leading "--" that `go run` may pass through.
	if len(args) > 0 && args[0] == "--" {
		args = args[1:]
	}
	if len(args) != 1 {
		fmt.Fprintln(os.Stderr, "usage: go run split_client.go -- path/to/client.go")
		os.Exit(2)
	}
	src := args[0]
	endpointsPath := filepath.Join(filepath.Dir(src), "endpoints.go")

	data, err := os.ReadFile(src)
	if err != nil {
		fail(err)
	}
	lines := strings.Split(string(data), "\n")

	// 1. Locate the Hostname-to-var-block region.
	hostnameStart := -1
	for i, line := range lines {
		if strings.HasPrefix(line, "// Hostname is the default hostname") {
			hostnameStart = i
			break
		}
	}
	if hostnameStart < 0 {
		fail(fmt.Errorf("%s: no Hostname comment found", src))
	}

	i := hostnameStart
	sawVar := false
	for i < len(lines) {
		switch {
		case lines[i] == "var (":
			sawVar = true
		case sawVar && lines[i] == ")":
			i++
			goto endHostname
		}
		i++
	}
endHostname:
	hostnameEnd := i // exclusive
	// Eat one trailing blank line.
	if hostnameEnd < len(lines) && lines[hostnameEnd] == "" {
		hostnameEnd++
	}
	hostnameBlock := strings.TrimRight(strings.Join(lines[hostnameStart:hostnameEnd], "\n"), "\n")

	// 2. Find all In/Out struct blocks: comment line + type ... { ... }
	type rng struct{ start, end int }
	var inoutRanges []rng
	commentRe := regexp.MustCompile(`^// ([A-Z][A-Za-z0-9_]+(?:In|Out)) (are|holds|packs|is)`)
	for i, line := range lines {
		m := commentRe.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		name := m[1]
		if i+1 >= len(lines) {
			continue
		}
		if !strings.HasPrefix(lines[i+1], "type "+name+" struct") {
			continue
		}
		start := i
		j := i + 1
		for j < len(lines) && lines[j] != "}" {
			j++
		}
		j++ // include closing }
		// Eat one trailing blank line.
		if j < len(lines) && lines[j] == "" {
			j++
		}
		inoutRanges = append(inoutRanges, rng{start, j})
	}

	// 3. Determine package name.
	pkgRe := regexp.MustCompile(`^package (\S+)`)
	pkgMatch := pkgRe.FindStringSubmatch(lines[0])
	if pkgMatch == nil {
		fail(fmt.Errorf("%s: no package declaration on line 1", src))
	}
	packageName := pkgMatch[1]

	// 4. Build endpoints.go body.
	blocks := []string{hostnameBlock}
	for _, r := range inoutRanges {
		blocks = append(blocks, strings.TrimRight(strings.Join(lines[r.start:r.end], "\n"), "\n"))
	}
	body := strings.Join(blocks, "\n\n") + "\n"

	// 5. Determine extra imports required by moved content.
	needsTime := regexp.MustCompile(`\btime\.(Time|Duration)\b`).MatchString(body)
	needsWorkflow := regexp.MustCompile(`\bworkflow\.Flow\b`).MatchString(body)

	var extraStd, extraMicrobus []string
	if needsTime {
		extraStd = append(extraStd, "\t\"time\"")
	}
	extraMicrobus = append(extraMicrobus, "\t\"github.com/microbus-io/fabric/httpx\"")
	if needsWorkflow {
		extraMicrobus = append(extraMicrobus, "\t\"github.com/microbus-io/fabric/workflow\"")
	}
	var importLines []string
	if len(extraStd) > 0 {
		importLines = append(importLines, extraStd...)
		importLines = append(importLines, "")
	}
	importLines = append(importLines, extraMicrobus...)
	importBlock := "import (\n" + strings.Join(importLines, "\n") + "\n)"

	endpointsContent := fmt.Sprintf("package %s\n\n%s\n\n%s", packageName, importBlock, body)
	if err := os.WriteFile(endpointsPath, []byte(endpointsContent), 0644); err != nil {
		fail(err)
	}

	// 6. Build new client.go by removing the moved ranges (in reverse order).
	allRemove := append([]rng{{hostnameStart, hostnameEnd}}, inoutRanges...)
	// Sort descending by start to delete safely.
	for a := 0; a < len(allRemove); a++ {
		for b := a + 1; b < len(allRemove); b++ {
			if allRemove[b].start > allRemove[a].start {
				allRemove[a], allRemove[b] = allRemove[b], allRemove[a]
			}
		}
	}
	newLines := append([]string(nil), lines...)
	for _, r := range allRemove {
		newLines = append(newLines[:r.start], newLines[r.end:]...)
	}

	// Collapse runs of >=3 blank lines to 2 to keep gofmt happy.
	out := make([]string, 0, len(newLines))
	blankRun := 0
	for _, ln := range newLines {
		if ln == "" {
			blankRun++
			if blankRun <= 2 {
				out = append(out, ln)
			}
		} else {
			blankRun = 0
			out = append(out, ln)
		}
	}
	newClient := strings.Join(out, "\n")
	if err := os.WriteFile(src, []byte(newClient), 0644); err != nil {
		fail(err)
	}

	fmt.Printf("OK %s -> %s\n", src, endpointsPath)
	fmt.Printf("  hostname_block: %d..%d\n", hostnameStart+1, hostnameEnd)
	fmt.Printf("  in/out ranges: %d\n", len(inoutRanges))
	fmt.Printf("  needs_time=%v needs_workflow=%v\n", needsTime, needsWorkflow)
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
