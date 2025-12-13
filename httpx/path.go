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

package httpx

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/microbus-io/errors"
	"github.com/microbus-io/fabric/utils"
)

// JoinHostAndPath combines the path shorthand with a hostname.
func JoinHostAndPath(host string, path string) string {
	if path == "" {
		// (empty)
		return "https://" + host
	}
	if strings.HasPrefix(path, ":") {
		// :1080/path
		return "https://" + host + path
	}
	if strings.HasPrefix(path, "//") {
		// //host.name/path/with/slash
		return "https:" + path
	}
	if strings.HasPrefix(path, "/") {
		// /path/with/slash
		return "https://" + host + path
	}
	if !strings.Contains(path, "://") {
		// path/with/no/slash
		return "https://" + host + "/" + path
	}
	return path
}

// ResolveURL resolves a URL in relation to the endpoint's base path.
func ResolveURL(base string, relative string) (resolved string, err error) {
	if relative == "" {
		return base, nil
	}
	baseURL, err := url.Parse(base)
	if err != nil {
		return "", errors.Trace(err)
	}
	relativeURL, err := url.Parse(relative)
	if err != nil {
		return "", errors.Trace(err)
	}
	resolvedURL := baseURL.ResolveReference(relativeURL)
	result := resolvedURL.String()
	result = strings.ReplaceAll(result, "%7B", "{")
	result = strings.ReplaceAll(result, "%7D", "}")
	return result, nil
}

// ParseURL returns a canonical version of the parsed URL with the scheme and port filled in if omitted.
// This method should only be used on internal URLs because it limits the hostname to certain specifications.
func ParseURL(rawURL string) (canonical *url.URL, err error) {
	if strings.Contains(rawURL, "`") {
		return nil, errors.New("backtick not allowed in URL '%s'", rawURL)
	}
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return nil, errors.Trace(err)
	}
	if err := ValidateHostname(parsed.Hostname()); err != nil {
		return nil, errors.Trace(err)
	}
	if parsed.Scheme == "" {
		parsed.Scheme = "https"
	}
	if parsed.Port() == "" {
		port := "443"
		if parsed.Scheme == "http" {
			port = "80"
		}
		parsed.Host += ":" + port
	}
	return parsed, nil
}

// InsertPathArguments fills URL path arguments such as {arg} from the named value.
// If the argument is not named, e.g. {}, then a default name of path1, path2, etc. is assumed.
//
// Deprecated: No longer used.
func InsertPathArguments(u string, values QArgs) string {
	if !strings.Contains(u, "{") {
		return u
	}
	parts := strings.Split(u, "/")
	argIndex := 0
	for i := range parts {
		if !strings.HasPrefix(parts[i], "{") || !strings.HasSuffix(parts[i], "}") {
			continue
		}
		argIndex++
		parts[i] = strings.TrimPrefix(parts[i], "{")
		parts[i] = strings.TrimSuffix(parts[i], "}")
		parts[i] = strings.TrimSuffix(parts[i], "+")
		if parts[i] == "" {
			parts[i] = fmt.Sprintf("path%d", argIndex)
		}
		if v, ok := values[parts[i]]; ok {
			parts[i] = url.PathEscape(utils.AnyToString(v))
		} else {
			parts[i] = ""
		}
	}
	return strings.Join(parts, "/")
}

// FillPathArguments transfers query arguments into path arguments, if present.
func FillPathArguments(u string) (resolved string, err error) {
	if !strings.Contains(u, "{") {
		return u, nil
	}
	var query url.Values
	u, q, found := strings.Cut(u, "?")
	if found {
		query, err = url.ParseQuery(q)
		if err != nil {
			return "", errors.Trace(err)
		}
	}
	parts := strings.Split(u, "/")
	argIndex := 0
	for i := range parts {
		if !strings.HasPrefix(parts[i], "{") || !strings.HasSuffix(parts[i], "}") {
			continue
		}
		greedy := strings.HasSuffix(parts[i], "+}")
		argIndex++
		parts[i] = strings.TrimPrefix(parts[i], "{")
		parts[i] = strings.TrimSuffix(parts[i], "}")
		parts[i] = strings.TrimSuffix(parts[i], "+")
		if parts[i] == "" {
			parts[i] = fmt.Sprintf("path%d", argIndex)
		}
		if vv, ok := query[parts[i]]; ok && len(vv) > 0 {
			delete(query, parts[i])
			v := vv[len(vv)-1]
			parts[i] = url.PathEscape(utils.AnyToString(v))
			if greedy {
				// Allow slashes in greedy arguments
				parts[i] = strings.ReplaceAll(parts[i], "%2F", "/")
			}
		} else {
			parts[i] = ""
		}
	}
	u = strings.Join(parts, "/")
	if len(query) > 0 {
		u += "?" + query.Encode()
	}
	return u, nil
}

// ExtractPathArguments extracts path arguments from a URL or path given a spec such as /obj/{id}/{} that identified them.
// Unnamed args are assigned the names path1, path2, etc.
//
// Deprecated: No longer used.
func ExtractPathArguments(spec string, path string) (args url.Values, err error) {
	if !strings.Contains(spec, "{") {
		return nil, nil
	}
	if _, after, cut := strings.Cut(spec, "://"); cut {
		spec = after
		if _, after, cut = strings.Cut(spec, "/"); cut {
			spec = after
			spec = "/" + spec
		}
	}
	if _, after, cut := strings.Cut(path, "://"); cut {
		path = after
		if _, after, cut = strings.Cut(path, "/"); cut {
			path = after
			path = "/" + path
		}
	}
	pathParts := strings.Split(path, "/")
	specParts := strings.Split(spec, "/")
	argIndex := 0
	for i := range specParts {
		if i >= len(pathParts) {
			break
		}
		if !strings.HasPrefix(specParts[i], "{") || !strings.HasSuffix(specParts[i], "}") {
			continue
		}
		argIndex++
		if pathParts[i] == specParts[i] {
			// No value provided in path
			continue
		}
		if args == nil {
			args = make(url.Values)
		}
		name := specParts[i]
		name = strings.TrimPrefix(name, "{")
		name = strings.TrimSuffix(name, "}")
		name = strings.TrimSuffix(name, "+")
		if name == "" {
			name = fmt.Sprintf("path%d", argIndex)
		}
		// Capture path appendix, e.g. /directory/{filename+}
		if i == len(specParts)-1 && strings.HasSuffix(specParts[i], "+}") {
			args.Set(name, strings.Join(pathParts[i:], "/"))
			break
		}
		args.Set(name, pathParts[i])
	}
	return args, nil
}

// SetPathValues compares the request's actual path against a parameterized route path such as /obj/{id}/{},
// and sets the path values of the request appropriately.
func SetPathValues(r *http.Request, routePath string) error {
	if !strings.Contains(routePath, "{") {
		return nil
	}
	unnamedIndex := 0
	routeParts := strings.Split(routePath, "/")
	actualParts := strings.Split(r.URL.Path, "/")
	for i := range routeParts {
		if !strings.HasPrefix(routeParts[i], "{") || !strings.HasSuffix(routeParts[i], "}") {
			continue
		}
		unnamedIndex++
		if i >= len(actualParts) {
			break
		}
		greedy := strings.HasSuffix(routeParts[i], "+}")
		routeParts[i] = strings.TrimPrefix(routeParts[i], "{")
		routeParts[i] = strings.TrimSuffix(routeParts[i], "}")
		routeParts[i] = strings.TrimSuffix(routeParts[i], "+")
		routeParts[i] = strings.TrimSpace(routeParts[i])
		if routeParts[i] == "" {
			routeParts[i] = fmt.Sprintf("path%d", unnamedIndex)
		}
		if greedy {
			r.SetPathValue(routeParts[i], strings.Join(actualParts[i:], "/"))
		} else {
			r.SetPathValue(routeParts[i], actualParts[i])
		}
	}
	return nil
}

// PathValues pulls the path values from the request given a parameterized route path such as /obj/{id}/{}.
// The path values should have been previously set on the request with SetPathValue.
func PathValues(r *http.Request, routePath string) (result url.Values, err error) {
	if !strings.Contains(routePath, "{") {
		return nil, nil
	}
	unnamedIndex := 0
	routeParts := strings.Split(routePath, "/")
	for i := range routeParts {
		if !strings.HasPrefix(routeParts[i], "{") || !strings.HasSuffix(routeParts[i], "}") {
			continue
		}
		unnamedIndex++
		routeParts[i] = strings.TrimPrefix(routeParts[i], "{")
		routeParts[i] = strings.TrimSuffix(routeParts[i], "}")
		routeParts[i] = strings.TrimSuffix(routeParts[i], "+")
		if routeParts[i] == "" {
			routeParts[i] = fmt.Sprintf("path%d", unnamedIndex)
		}
		val := r.PathValue(routeParts[i])
		if val != "" {
			if result == nil {
				result = url.Values{}
			}
			result[routeParts[i]] = []string{val}
		}
	}
	return result, nil
}
