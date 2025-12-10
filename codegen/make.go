/*
Copyright (c) 2023-2025 Microbus LLC and various contributors

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
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/microbus-io/fabric/codegen/spec"
	"github.com/microbus-io/fabric/errors"
	"github.com/microbus-io/fabric/utils"
)

// makeIntegration creates the integration tests.
func (gen *Generator) makeIntegration() (err error) {
	gen.Printer.Debug("Generating integration tests")
	gen.Printer.Indent()
	defer gen.Printer.Unindent()

	if !gen.Specs.General.IntegrationTests {
		gen.Printer.Debug("Disabled in service.yaml")
		return nil
	}

	// Fully qualify the types outside of the API directory
	gen.Specs.FullyQualifyTypes()

	// Create service_test.go if it doesn't exist
	fileName := filepath.Join(gen.WorkDir, "service_test.go")
	_, err = os.Stat(fileName)
	if errors.Is(err, os.ErrNotExist) {
		tt, err := LoadTemplate("service/service_test.txt")
		if err != nil {
			return errors.Trace(err)
		}
		err = tt.Overwrite(fileName, gen.Specs)
		if err != nil {
			return errors.Trace(err)
		}
		gen.Printer.Debug("service_test.go")
	}

	// Scan .go files for existing endpoints
	gen.Printer.Debug("Scanning for existing tests")
	pkg := capitalizeIdentifier(gen.Specs.PackagePathSuffix())
	existingTests, err := gen.scanFiles(
		gen.WorkDir,
		func(file fs.DirEntry) bool {
			return strings.HasSuffix(file.Name(), "_test.go") &&
				!strings.HasSuffix(file.Name(), "-gen_test.go")
		},
		`func Test`+pkg+`_([A-Z][a-zA-Z0-9]*)\(t `,
	) // func TestService_XXX(t *testing.T)
	if err != nil {
		return errors.Trace(err)
	}
	gen.Printer.Indent()
	for k := range existingTests {
		gen.Printer.Debug(k)
	}
	gen.Printer.Unindent()

	// Mark existing tests in the specs
	newTests := false
	for _, h := range gen.Specs.AllHandlers() {
		if h.Type != "function" && h.Type != "event" && h.Type != "sink" && h.Type != "web" && h.Type != "ticker" && h.Type != "config" && h.Type != "metric" {
			continue
		}
		if existingTests[h.Name()] ||
			existingTests["OnChanged"+h.Name()] ||
			existingTests["OnObserve"+h.Name()] {
			h.Exists = true
		} else {
			h.Exists = false
			newTests = true
		}
	}

	// Append new handlers
	fileName = filepath.Join(gen.WorkDir, "service_test.go")
	if newTests {
		tt, err := LoadTemplate("service/service_test.append.txt")
		if err != nil {
			return errors.Trace(err)
		}
		err = tt.AppendTo(fileName, gen.Specs)
		if err != nil {
			return errors.Trace(err)
		}

		gen.Printer.Debug("New tests created")
		gen.Printer.Indent()
		for _, h := range gen.Specs.AllHandlers() {
			if h.Type != "function" && h.Type != "event" && h.Type != "sink" && h.Type != "web" && h.Type != "ticker" && h.Type != "metric" {
				continue
			}
			if !h.Exists {
				gen.Printer.Debug("%s", h.Name())
			}
		}
		gen.Printer.Unindent()
	}

	// Add imports to new event sources
	if len(gen.Specs.Sinks) > 0 {
		content, err := os.ReadFile(fileName)
		if err != nil {
			return errors.Trace(err)
		}
		modified := false
		for _, h := range gen.Specs.Sinks {
			if h.Exists {
				continue
			}
			if bytes.Index(content, []byte(`"`+h.Source+"/"+h.SourceSuffix()+"api"+`"`)) > 0 {
				// Already imported
				continue
			}

			findImportPointer := func() int {
				p := bytes.Index(content, []byte("\nimport ("))
				if p < 0 {
					return -1
				}
				q := bytes.Index(content[p:], []byte("\n)"))
				if q < 0 {
					return -1
				}
				return p + q + 1 // At start of row
			}
			p1 := findImportPointer()
			if p1 < 0 {
				continue
			}

			// Add import statement
			var buf bytes.Buffer
			buf.Write(content[:p1])
			buf.Write([]byte("\t\"" + h.Source + "/" + h.SourceSuffix() + "api\"\n"))
			buf.Write(content[p1:])
			content = buf.Bytes()
			modified = true
		}
		if modified {
			err = os.WriteFile(fileName, content, 0666)
			if err != nil {
				return errors.Trace(err)
			}
		}
	}

	return nil
}

// makeIntermediate creates the intermediate directory and files.
func (gen *Generator) makeIntermediate() error {
	gen.Printer.Debug("Generating intermediate")
	gen.Printer.Indent()
	defer gen.Printer.Unindent()

	// Fully qualify the types outside of the API directory
	gen.Specs.FullyQualifyTypes()

	// Create the directory
	_, err := gen.mkdir("intermediate")
	if err != nil {
		return errors.Trace(err)
	}

	// intermediate-gen.go
	fileName := filepath.Join(gen.WorkDir, "intermediate", "intermediate-gen.go")
	tt, err := LoadTemplate(
		"service/intermediate/intermediate-gen.txt",
		"service/intermediate/intermediate-gen.configs.txt",
		"service/intermediate/intermediate-gen.functions.txt",
		"service/intermediate/intermediate-gen.metrics.txt",
	)
	if err != nil {
		return errors.Trace(err)
	}
	err = tt.Overwrite(fileName, gen.Specs)
	if err != nil {
		return errors.Trace(err)
	}
	gen.Printer.Debug("intermediate/intermediate-gen.go")

	// mock-gen.go
	fileName = filepath.Join(gen.WorkDir, "intermediate", "mock-gen.go")
	tt, err = LoadTemplate(
		"service/intermediate/mock-gen.txt",
	)
	if err != nil {
		return errors.Trace(err)
	}
	err = tt.Overwrite(fileName, gen.Specs)
	if err != nil {
		return errors.Trace(err)
	}
	gen.Printer.Debug("intermediate/mock-gen.go")
	return nil
}

// makeResources creates the resources directory and files.
func (gen *Generator) makeResources() error {
	gen.Printer.Debug("Generating resources")
	gen.Printer.Indent()
	defer gen.Printer.Unindent()

	// Create the directory
	_, err := gen.mkdir("resources")
	if err != nil {
		return errors.Trace(err)
	}

	// embed-gen.go
	fileName := filepath.Join(gen.WorkDir, "resources", "embed-gen.go")
	tt, err := LoadTemplate("service/resources/embed-gen.txt")
	if err != nil {
		return errors.Trace(err)
	}
	err = tt.Overwrite(fileName, gen.Specs)
	if err != nil {
		return errors.Trace(err)
	}
	gen.Printer.Debug("resources/embed-gen.go")

	return nil
}

// makeAPI creates the API directory and files.
func (gen *Generator) makeAPI() error {
	gen.Printer.Debug("Generating client API")
	gen.Printer.Indent()
	defer gen.Printer.Unindent()

	// Should not fully qualify types when generating inside the API directory
	gen.Specs.ShorthandTypes()

	// Create the directory
	_, err := gen.mkdir(gen.Specs.PackageDirSuffix() + "api")
	if err != nil {
		return errors.Trace(err)
	}
	dir := filepath.Join(gen.WorkDir, gen.Specs.PackageDirSuffix()+"api")

	// Types
	if len(gen.Specs.Types) > 0 {
		// Scan .go files for existing types
		gen.Printer.Debug("Scanning for existing types")
		existingTypes, err := gen.scanFiles(
			dir,
			func(file fs.DirEntry) bool {
				return strings.HasSuffix(file.Name(), ".go") &&
					!strings.HasSuffix(file.Name(), "_test.go") &&
					!strings.HasSuffix(file.Name(), "-gen.go")
			},
			`type ([A-Z][a-zA-Z0-9]*) `, // type XXX
		)
		if err != nil {
			return errors.Trace(err)
		}
		gen.Printer.Indent()
		for k := range existingTypes {
			gen.Printer.Debug(k)
		}
		gen.Printer.Unindent()

		// Mark existing types in the specs
		newTypes := false
		for _, ct := range gen.Specs.Types {
			ct.Exists = existingTypes[ct.Name]
			newTypes = newTypes || !ct.Exists
		}

		// Append new type definitions
		if newTypes {
			// Scan entire project type definitions and try to resolve new types
			typeDefs, err := gen.scanProjectTypeDefinitions()
			if err != nil {
				return errors.Trace(err)
			}
			hasImports := false
			for _, ct := range gen.Specs.Types {
				if !ct.Exists && len(typeDefs[ct.Name]) == 1 {
					ct.PackagePath = typeDefs[ct.Name][0]
					hasImports = true
				}
			}
			fileName := filepath.Join(dir, "imports-gen.go")
			if hasImports {
				// Create imports-gen.go
				tt, err := LoadTemplate("service/api/imports-gen.txt")
				if err != nil {
					return errors.Trace(err)
				}
				err = tt.Overwrite(fileName, gen.Specs)
				if err != nil {
					return errors.Trace(err)
				}
				gen.Printer.Debug("%sapi/imports-gen.go", gen.Specs.PackageDirSuffix())
			} else {
				os.Remove(fileName)
			}

			// Create a file for each new type
			for _, ct := range gen.Specs.Types {
				if !ct.Exists && len(typeDefs[ct.Name]) != 1 {
					fileName := filepath.Join(dir, strings.ToLower(ct.Name)+".go")
					tt, err := LoadTemplate("service/api/type.txt")
					if err != nil {
						return errors.Trace(err)
					}
					ct.PackagePath = gen.Specs.PackagePath // Hack
					err = tt.Overwrite(fileName, ct)
					if err != nil {
						return errors.Trace(err)
					}
					gen.Printer.Debug("%sapi/%s.go", gen.Specs.PackageDirSuffix(), strings.ToLower(ct.Name))
				}
			}
		}
	}

	// clients-gen.go
	fileName := filepath.Join(gen.WorkDir, gen.Specs.PackageDirSuffix()+"api", "clients-gen.go")
	tt, err := LoadTemplate(
		"service/api/clients-gen.txt",
		"service/api/clients-gen.webs.txt",
		"service/api/clients-gen.functions.txt",
	)
	if err != nil {
		return errors.Trace(err)
	}
	err = tt.Overwrite(fileName, gen.Specs)
	if err != nil {
		return errors.Trace(err)
	}
	gen.Printer.Debug("%sapi/clients-gen.go", gen.Specs.PackageDirSuffix())

	return nil
}

// makeImplementation generates service.go and service-gen.go.
func (gen *Generator) makeImplementation() error {
	gen.Printer.Debug("Generating implementation")
	gen.Printer.Indent()
	defer gen.Printer.Unindent()

	// Fully qualify the types outside of the API directory
	gen.Specs.FullyQualifyTypes()

	// Overwrite service-gen.go
	fileName := filepath.Join(gen.WorkDir, "service-gen.go")
	tt, err := LoadTemplate("service/service-gen.txt")
	if err != nil {
		return errors.Trace(err)
	}
	err = tt.Overwrite(fileName, gen.Specs)
	if err != nil {
		return errors.Trace(err)
	}
	gen.Printer.Debug("service-gen.go")

	// Create service.go if it doesn't exist
	fileName = filepath.Join(gen.WorkDir, "service.go")
	_, err = os.Stat(fileName)
	if errors.Is(err, os.ErrNotExist) {
		tt, err = LoadTemplate("service/service.txt")
		if err != nil {
			return errors.Trace(err)
		}
		err := tt.Overwrite(fileName, gen.Specs)
		if err != nil {
			return errors.Trace(err)
		}
		gen.Printer.Debug("service.go")

		err = gen.makeAddToMainApp()
		if err != nil {
			return errors.Trace(err)
		}
	}

	// Create AGENTS.md and CLAUDE.md if they do not exist
	for _, filename := range []string{"AGENTS.md", "CLAUDE.md"} {
		fullPath := filepath.Join(gen.WorkDir, filename)
		_, err = os.Stat(fullPath)
		if errors.Is(err, os.ErrNotExist) {
			tt, err = LoadTemplate("service/" + filename)
			if err != nil {
				return errors.Trace(err)
			}
			err := tt.Overwrite(fullPath, gen.Specs)
			if err != nil {
				return errors.Trace(err)
			}
			gen.Printer.Debug(filename)
		}
	}

	// Scan .go files for existing endpoints
	gen.Printer.Debug("Scanning for existing handlers")
	existingEndpoints, err := gen.scanFiles(
		gen.WorkDir,
		func(file fs.DirEntry) bool {
			return strings.HasSuffix(file.Name(), ".go") &&
				!strings.HasSuffix(file.Name(), "_test.go") &&
				!strings.HasSuffix(file.Name(), "-gen.go")
		},
		`func \(svc \*Service\) ([A-Z][a-zA-Z0-9]*)\(`, // func (svc *Service) XXX(
	)
	if err != nil {
		return errors.Trace(err)
	}
	gen.Printer.Indent()
	for k := range existingEndpoints {
		gen.Printer.Debug(k)
	}
	gen.Printer.Unindent()

	// Mark existing handlers in the specs
	newEndpoints := false
	for _, h := range gen.Specs.AllHandlers() {
		if h.Type == "config" && !h.Callback {
			continue
		}
		if h.Type == "metric" && !h.Callback {
			continue
		}
		if existingEndpoints[h.Name()] ||
			existingEndpoints["OnChanged"+h.Name()] ||
			existingEndpoints["OnObserve"+h.Name()] {
			h.Exists = true
		} else {
			h.Exists = false
			newEndpoints = true
		}
	}

	// Append new handlers
	fileName = filepath.Join(gen.WorkDir, "service.go")
	if newEndpoints {
		tt, err = LoadTemplate("service/service.append.txt")
		if err != nil {
			return errors.Trace(err)
		}
		err = tt.AppendTo(fileName, gen.Specs)
		if err != nil {
			return errors.Trace(err)
		}

		gen.Printer.Debug("New handlers created")
		gen.Printer.Indent()
		for _, h := range gen.Specs.AllHandlers() {
			if h.Type == "config" && !h.Callback {
				continue
			}
			if h.Type == "metric" && !h.Callback {
				continue
			}
			if !h.Exists {
				switch h.Type {
				case "config":
					gen.Printer.Debug("OnChanged%s", h.Name())
				case "metric":
					gen.Printer.Debug("OnObserve%s", h.Name())
				default:
					gen.Printer.Debug("%s", h.Name())
				}
			}
		}
		gen.Printer.Unindent()
	}

	return nil
}

// scanFiles scans all files with the indicated suffix for all sub-matches of the regular expression.
func (gen *Generator) scanFiles(workDir string, filter func(file fs.DirEntry) bool, regExpression string) (map[string]bool, error) {
	result := map[string]bool{}
	re, err := regexp.Compile(regExpression)
	if err != nil {
		return nil, errors.Trace(err)
	}
	files, err := os.ReadDir(workDir)
	if err != nil {
		return nil, errors.Trace(err)
	}
	for _, file := range files {
		if file.IsDir() || !filter(file) {
			continue
		}
		fileName := filepath.Join(workDir, file.Name())
		body, err := os.ReadFile(fileName)
		if err != nil {
			return nil, errors.Trace(err)
		}
		allSubMatches := re.FindAllStringSubmatch(string(body), -1)
		for _, subMatches := range allSubMatches {
			if len(subMatches) == 2 {
				result[subMatches[1]] = true
			}
		}
	}
	return result, nil
}

// scanProjectTypeDefinitions scans for type definitions in the entire project tree.
func (gen *Generator) scanProjectTypeDefinitions() (map[string][]string, error) {
	found := map[string][]string{}
	gen.Printer.Debug("Scanning project type definitions")
	gen.Printer.Indent()
	defer gen.Printer.Unindent()
	err := gen.scanDirTypeDefinitions(gen.ProjectDir, found)
	if err != nil {
		return nil, errors.Trace(err)
	}
	return found, nil
}

// scanDirTypeDefinitions scans for type definitions in a directory tree.
func (gen *Generator) scanDirTypeDefinitions(workDir string, found map[string][]string) error {
	// Skip directories starting with .
	if strings.HasPrefix(filepath.Base(workDir), ".") {
		return nil
	}
	// Detect if processing a service directory
	_, err := os.Stat(filepath.Join(workDir, "service.yaml"))
	serviceDirectory := err == nil

	if !serviceDirectory {
		// Scan for type definitions in Go files
		typeDefs, err := gen.scanFiles(
			workDir,
			func(file fs.DirEntry) bool {
				return strings.HasSuffix(file.Name(), ".go") &&
					!strings.HasSuffix(file.Name(), "_test.go") &&
					!strings.HasSuffix(file.Name(), "-gen.go")
			},
			`type ([A-Z][a-zA-Z0-9]*) [^=]`, // type XXX struct, type XXX int, etc.
		)
		if err != nil {
			return errors.Trace(err)
		}
		if len(typeDefs) > 0 {
			subPath := strings.TrimPrefix(workDir, gen.ProjectDir)
			pkg := strings.ReplaceAll(filepath.Join(gen.ModulePath, subPath), "\\", "/")
			gen.Printer.Debug(pkg)
			gen.Printer.Indent()
			for k := range typeDefs {
				gen.Printer.Debug(k)
				found[k] = append(found[k], pkg)
			}
			gen.Printer.Unindent()
		}
	}

	// Recurse into sub directories
	files, err := os.ReadDir(workDir)
	if err != nil {
		return errors.Trace(err)
	}
	for _, file := range files {
		if file.IsDir() {
			if serviceDirectory &&
				(file.Name() == "intermediate" || file.Name() == "resources" || file.Name() == "app") {
				continue
			}
			err = gen.scanDirTypeDefinitions(filepath.Join(workDir, file.Name()), found)
			if err != nil {
				return errors.Trace(err)
			}
		}
	}
	return nil
}

// makeTraceReturnedErrors adds errors.Trace to returned errors.
func (gen *Generator) makeTraceReturnedErrors() error {
	gen.Printer.Debug("Adding tracing to returned errors")
	gen.Printer.Indent()
	defer gen.Printer.Unindent()

	err := gen.makeTraceReturnedErrorsDir(gen.WorkDir)
	if err != nil {
		return errors.Trace(err)
	}
	return nil
}

func (gen *Generator) makeTraceReturnedErrorsDir(directory string) error {
	files, err := os.ReadDir(directory)
	if err != nil {
		return errors.Trace(err)
	}
	for _, file := range files {
		fileName := filepath.Join(directory, file.Name())
		if file.IsDir() {
			if file.Name() == "intermediate" || file.Name() == "resources" {
				continue
			}
			err = gen.makeTraceReturnedErrorsDir(fileName)
			if err != nil {
				return errors.Trace(err)
			}
		}
		if !strings.HasSuffix(file.Name(), ".go") ||
			strings.HasSuffix(file.Name(), "_test.go") ||
			strings.HasSuffix(file.Name(), "-gen.go") {
			continue
		}

		buf, err := os.ReadFile(fileName)
		if err != nil {
			return errors.Trace(err)
		}
		body := string(buf)
		alteredBody := findReplaceReturnedErrors(body)
		alteredBody = findReplaceImportErrors(alteredBody)
		if body != alteredBody {
			err = os.WriteFile(fileName, []byte(alteredBody), 0666)
			if err != nil {
				return errors.Trace(err)
			}
			gen.Printer.Debug("%s", strings.TrimLeft(fileName, gen.WorkDir+"/"))
		}
	}

	return nil
}

var traceReturnErrRegexp = regexp.MustCompile(`\n(\s*)return ([^\n]+, )?(err)\n`)

func findReplaceReturnedErrors(body string) (modified string) {
	return traceReturnErrRegexp.ReplaceAllString(body, "\n${1}return ${2}errors.Trace(err)\n")
}

func findReplaceImportErrors(body string) (modified string) {
	hasTracing := strings.Contains(body, "errors.Trace(")

	modified = strings.ReplaceAll(body, "\n"+`import "errors"`+"\n", "\n"+`import "github.com/microbus-io/fabric/errors"`+"\n")

	start := strings.Index(modified, "\nimport (\n")
	if start < 0 {
		return modified
	}
	end := strings.Index(modified[start:], ")")
	if end < 0 {
		return modified
	}

	var result strings.Builder
	result.WriteString(modified[:start])

	stmt := modified[start : start+end+1]
	lines := strings.Split(stmt, "\n")
	whitespace := "\t"
	goErrorsFound := false
	microbusErrorsFound := false
	for i, line := range lines {
		if strings.HasSuffix(line, `"errors"`) {
			whitespace = strings.TrimSuffix(line, `"errors"`)
			goErrorsFound = true
			continue
		}
		if strings.HasSuffix(line, `"github.com/microbus-io/fabric/errors"`) {
			microbusErrorsFound = true
		}
		if line == ")" && (goErrorsFound || hasTracing) && !microbusErrorsFound {
			if i >= 2 && lines[i-2] != "import (" {
				result.WriteString("\n")
			}
			result.WriteString(whitespace)
			result.WriteString(`"github.com/microbus-io/fabric/errors"`)
			result.WriteString("\n)\n")
		} else {
			result.WriteString(line)
			result.WriteString("\n")
		}
	}
	result.WriteString(modified[start+end+2:])
	return result.String()
}

func (gen *Generator) makeRefreshSignature() error {
	gen.Printer.Debug("Refreshing handler signatures")
	gen.Printer.Indent()
	defer gen.Printer.Unindent()

	// Fully qualify the types outside of the API directory
	gen.Specs.FullyQualifyTypes()

	files, err := os.ReadDir(gen.WorkDir)
	if err != nil {
		return errors.Trace(err)
	}
	for _, file := range files {
		if file.IsDir() {
			continue
		}
		if !strings.HasSuffix(file.Name(), ".go") ||
			strings.HasSuffix(file.Name(), "_test.go") ||
			strings.HasSuffix(file.Name(), "-gen.go") {
			continue
		}
		fileName := filepath.Join(gen.WorkDir, file.Name())
		buf, err := os.ReadFile(fileName)
		if err != nil {
			return errors.Trace(err)
		}
		body := string(buf)
		alteredBody := findReplaceSignature(gen.Specs, body)
		alteredBody = findReplaceDescription(gen.Specs, alteredBody)
		if body != alteredBody {
			err = os.WriteFile(fileName, []byte(alteredBody), 0666)
			if err != nil {
				return errors.Trace(err)
			}
			gen.Printer.Debug("%s", file.Name())
		}
	}

	return nil
}

// findReplaceSignature updates the signature of functions.
func findReplaceSignature(specs *spec.Service, source string) (modified string) {
	endpoints := []*spec.Handler{}
	endpoints = append(endpoints, specs.Functions...)
	endpoints = append(endpoints, specs.Sinks...)
	for _, fn := range endpoints {
		p := strings.Index(source, "func (svc *Service) "+fn.Name()+"(")
		if p < 0 {
			continue
		}
		fnSig := "func (svc *Service) " + fn.Name() + "(ctx context.Context" + fn.In(", name type") + ") (" + fn.Out("name type,") + "err error)"
		q := strings.Index(source[p:], fnSig)
		if q != 0 {
			// Signature changed
			nl := strings.Index(source[p:], " {")
			if nl >= 0 {
				source = strings.Replace(source, source[p:p+nl], fnSig, 1)
			}
		}
	}
	return source
}

// findReplaceDescription updates the description of handlers.
func findReplaceDescription(specs *spec.Service, source string) (modified string) {
	desc := fmt.Sprintf("Service implements the %s microservice.\n\n%s\n", specs.General.Host, specs.General.Description)
	desc = strings.TrimSpace(desc)
	source = findReplaceCommentBefore(source, "\ntype Service struct {", desc)

	for _, h := range specs.AllHandlers() {
		source = findReplaceCommentBefore(source, "\nfunc (svc *Service) "+h.Name()+"(", h.Description)
	}
	return source
}

// findReplaceCommentBefore updates the description of handlers.
func findReplaceCommentBefore(source string, searchTerm string, comment string) (modified string) {
	pos := strings.Index(source, searchTerm)
	if pos < 0 {
		return source
	}

	newComment := "/*\n" + comment + "\n*/"

	// /*
	// Comment
	// */
	// func (svc *Service) ...
	q := strings.LastIndex(source[:pos], "*/")
	if q == pos-len("*/") {
		q += len("*/")
		p := strings.LastIndex(source[:pos], "/*")
		if p > 0 && source[p:q] != newComment {
			source = source[:p] + newComment + source[q:]
		}
		return source
	}

	// // Comment
	// func (svc *Service) ...
	p := pos + 1
	q = pos
	for {
		q = strings.LastIndex(source[:q], "\n")
		if q < 0 || !strings.HasPrefix(source[q:], "\n//") {
			break
		}
		p = q + 1
	}
	source = source[:p] + newComment + source[pos:]
	return source
}

// makeVersion generates the versioning files.
func (gen *Generator) makeVersion(version int) error {
	gen.Printer.Debug("Versioning")
	gen.Printer.Indent()
	defer gen.Printer.Unindent()

	hash, err := utils.SourceCodeSHA256(gen.WorkDir)
	if err != nil {
		return errors.Trace(err)
	}

	v := &spec.Version{
		PackagePath: gen.Specs.PackagePath,
		Version:     version,
		Timestamp:   time.Now().UTC().Format(time.RFC3339Nano),
		SHA256:      hash,
	}
	gen.Printer.Debug("Version %d", v.Version)
	gen.Printer.Debug("SHA256 %s", v.SHA256)
	gen.Printer.Debug("Timestamp %v", v.Timestamp)

	tt, err := LoadTemplate("service/version-gen.txt")
	if err != nil {
		return errors.Trace(err)
	}
	fileName := filepath.Join(gen.WorkDir, "version-gen.go")
	err = tt.Overwrite(fileName, &v)
	if err != nil {
		return errors.Trace(err)
	}

	tt, err = LoadTemplate("service/version-gen_test.txt")
	if err != nil {
		return errors.Trace(err)
	}
	fileName = filepath.Join(gen.WorkDir, "version-gen_test.go")
	err = tt.Overwrite(fileName, &v)
	if err != nil {
		return errors.Trace(err)
	}

	return nil
}

// makeClaude populates the .claude directory with files.
func (gen *Generator) makeClaude() (err error) {
	gen.Printer.Debug("Generating .claude")
	gen.Printer.Indent()
	defer gen.Printer.Unindent()

	err = gen.copy(".claude", ".claude", true)
	if err != nil {
		return errors.Trace(err)
	}
	return nil
}

// makeRoot populates the root directory with files.
func (gen *Generator) makeRoot() (err error) {
	gen.Printer.Debug("Generating project root")
	gen.Printer.Indent()
	defer gen.Printer.Unindent()

	// Always overwrite
	err = gen.copy("AGENTS-MICROBUS.md", "AGENTS-MICROBUS.md", true)
	if err != nil {
		return errors.Trace(err)
	}

	// Add to coding assistant file
	for _, filename := range []string{"AGENTS.md", "CLAUDE.md"} {
		tt, err := LoadTemplate(filename)
		if err != nil {
			return errors.Trace(err)
		}
		fullPath := filepath.Join(gen.WorkDir, filename)
		_, err = os.Stat(fullPath)
		if errors.Is(err, os.ErrNotExist) {
			err = tt.Overwrite(filename, gen.Specs)
			if err != nil {
				return errors.Trace(err)
			}
			gen.Printer.Debug(filename)
		} else {
			content, err := os.ReadFile(filename)
			if err != nil {
				return errors.Trace(err)
			}
			executed, err := tt.Execute(gen.WorkDir)
			if err != nil {
				return errors.Trace(err)
			}
			if !bytes.Contains(content, executed) {
				err = tt.AppendTo(filename, gen.Specs)
				if err != nil {
					return errors.Trace(err)
				}
				gen.Printer.Debug(filename)
			}
		}
	}

	return nil
}

// makeMain creates the main directory and files.
func (gen *Generator) makeMain() (err error) {
	gen.Printer.Debug("Generating main")
	gen.Printer.Indent()
	defer gen.Printer.Unindent()

	// Create the directory
	created, err := gen.mkdir("main")
	if err != nil {
		return errors.Trace(err)
	}
	if created {
		err = gen.copy("main", "main", true)
		if err != nil {
			return errors.Trace(err)
		}
	}
	return nil
}

// makeVSCode creates the .vscode directory and files.
func (gen *Generator) makeVSCode() (err error) {
	gen.Printer.Debug("Generating .vscode")
	gen.Printer.Indent()
	defer gen.Printer.Unindent()

	err = gen.copy(".vscode", ".vscode", false)
	if err != nil {
		return errors.Trace(err)
	}
	return nil
}

// mkdir creates a directory, if it doesn't exist.
// The path is relative to the working directory.
func (gen *Generator) mkdir(dirPath string) (created bool, err error) {
	fullPath := filepath.Join(gen.WorkDir, dirPath)
	_, err = os.Stat(fullPath)
	if errors.Is(err, os.ErrNotExist) {
		err = os.Mkdir(fullPath, os.ModePerm)
		if err != nil {
			return false, errors.Trace(err)
		}
		gen.Printer.Debug("mkdir %s", dirPath)
		return true, nil
	} else if err != nil {
		return false, errors.Trace(err)
	}
	return false, nil
}

// copy copies a file from the resource to disk.
// The path is relative to the working directory.
func (gen *Generator) copy(filePath string, resourcePath string, overwrite bool) (err error) {
	// Detect if the resource is a file or a directory
	fsFile, err := bundle.Open(filepath.Join("bundle", resourcePath))
	if err != nil {
		return errors.Trace(err)
	}
	fsFileStat, err := fsFile.Stat()
	if err != nil {
		return errors.Trace(err)
	}

	// Copy directory
	if fsFileStat.IsDir() {
		_, err = gen.mkdir(filePath)
		if err != nil {
			return errors.Trace(err)
		}
		entries, err := bundle.ReadDir(filepath.Join("bundle", resourcePath))
		if err != nil {
			return errors.Trace(err)
		}
		for _, entry := range entries {
			entryBase := filepath.Base(entry.Name())
			err = gen.copy(filepath.Join(filePath, strings.TrimRight(entryBase, "_")), filepath.Join(resourcePath, entryBase), overwrite)
			if err != nil {
				return errors.Trace(err)
			}
		}
		return nil
	}

	fullFilePath := filepath.Join(gen.WorkDir, filePath)
	currentContent, err := os.ReadFile(fullFilePath)
	notExists := errors.Is(err, os.ErrNotExist)
	if err != nil && !notExists {
		return errors.Trace(err)
	}
	if !overwrite && !notExists {
		return nil
	}

	// Copy single file
	newContent, err := bundle.ReadFile(filepath.Join("bundle", resourcePath))
	if err != nil {
		return errors.Trace(err)
	}
	if bytes.Equal(newContent, currentContent) {
		return nil
	}
	file, err := os.Create(fullFilePath) // Overwrite
	if err != nil {
		return errors.Trace(err)
	}
	defer file.Close()
	_, err = file.Write(newContent)
	if err != nil {
		return errors.Trace(err)
	}
	gen.Printer.Debug("%s", filePath)
	return nil
}

// makeAddToMainApp adds the microservice to the main app.
func (gen *Generator) makeAddToMainApp() (err error) {
	gen.Printer.Debug("Adding to main app")
	gen.Printer.Indent()
	defer gen.Printer.Unindent()

	if testing.Testing() {
		gen.Printer.Debug("Skipped while testing")
		return nil
	}

	fileName := filepath.Join(gen.ProjectDir, "main/main.go")
	_, err = os.Stat(fileName)
	if errors.Is(err, os.ErrNotExist) {
		gen.Printer.Debug("main/main.go not found")
		return nil
	}

	shortPackageName := gen.PackagePath
	p := strings.LastIndex(gen.PackagePath, "/")
	if p >= 0 {
		shortPackageName = gen.PackagePath[p+1:]
	}

	content, err := os.ReadFile(fileName)
	if err != nil {
		return errors.Trace(err)
	}
	if bytes.Index(content, []byte(shortPackageName+".NewService()")) > 0 ||
		bytes.Index(content, []byte(gen.PackagePath)) > 0 {
		return nil
	}

	findImportPointer := func() int {
		p := bytes.Index(content, []byte("\nimport ("))
		if p < 0 {
			return -1
		}
		q := bytes.Index(content[p:], []byte("\n)"))
		if q < 0 {
			return -1
		}
		return p + q + 1 // At start of row
	}
	findNewServicePointer := func() int {
		p := bytes.Index(content, []byte("httpingress.NewService()"))
		if p < 0 {
			p = bytes.Index(content, []byte("\tapp.Run()"))
		}
		if p < 0 {
			return -1
		}
		q := bytes.LastIndex(content[:p], []byte("\n\tapp.Add("))
		if q < 0 {
			return -1
		}
		r := bytes.LastIndex(content[:q], []byte("\n\tapp.Add("))
		if r < 0 {
			r = q
		}
		s := bytes.Index(content[r:], []byte("\n\t)"))
		if s < 0 {
			return -1
		}
		return r + s + 1 // At start of row
	}

	p1 := findImportPointer()
	p2 := findNewServicePointer()
	if p1 < 0 || p2 < 0 {
		gen.Printer.Debug("Insert locations not found")
		return nil
	}

	// Add import statement
	var buf1 bytes.Buffer
	buf1.Write(content[:p1])
	buf1.Write([]byte("\t\"" + gen.PackagePath + "\"\n"))
	buf1.Write(content[p1:])
	content = buf1.Bytes()
	p2 += len([]byte("\t\"" + gen.PackagePath + "\"\n"))

	// Add shortpackage.NewService
	var buf2 bytes.Buffer
	buf2.Write(content[:p2])
	buf2.Write([]byte("\t\t" + shortPackageName + ".NewService(),\n"))
	buf2.Write(content[p2:])
	content = buf2.Bytes()

	err = os.WriteFile(fileName, content, 0666)
	if err != nil {
		return errors.Trace(err)
	}
	gen.Printer.Debug("%s", shortPackageName)
	return nil
}
