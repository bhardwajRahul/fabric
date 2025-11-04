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

package connector

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"os"
	"strconv"
	"strings"

	htmltemplate "html/template"
	texttemplate "text/template"

	"github.com/microbus-io/fabric/errors"
	"github.com/microbus-io/fabric/frame"
	"github.com/microbus-io/fabric/service"
	"github.com/microbus-io/fabric/utils"
	"gopkg.in/yaml.v3"
)

// SetResFS initialized the connector to load resource files from an arbitrary FS.
func (c *Connector) SetResFS(fs service.FS) error {
	if c.IsStarted() {
		return c.captureInitErr(errors.New("already started"))
	}
	c.resourcesFS = fs
	err := c.initStringBundle()
	if err != nil {
		return c.captureInitErr(errors.Trace(err))
	}
	return nil
}

// SetResFSDir initialized the connector to load resource files from a directory.
func (c *Connector) SetResFSDir(directoryPath string) error {
	err := c.SetResFS(os.DirFS(directoryPath).(service.FS)) // Casting required
	return errors.Trace(err)
}

// ResFS returns the FS associated with the connector.
func (c *Connector) ResFS() service.FS {
	return c.resourcesFS
}

// ReadResFile returns the content of a resource file.
func (c *Connector) ReadResFile(name string) ([]byte, error) {
	b, err := c.resourcesFS.ReadFile(name)
	return b, errors.Trace(err)
}

// MustReadResFile returns the content of a resource file, or nil if not found.
// It panics if the resource file is not found.
func (c *Connector) MustReadResFile(name string) []byte {
	b, err := c.resourcesFS.ReadFile(name)
	if err != nil {
		panic(err)
	}
	return b
}

// ReadResTextFile returns the content of a resource file as a string.
func (c *Connector) ReadResTextFile(name string) (string, error) {
	b, err := c.resourcesFS.ReadFile(name)
	return string(b), errors.Trace(err)
}

// MustReadResTextFile returns the content of a resource file as a string, or "" if not found.
// It panics if the resource file is not found.
func (c *Connector) MustReadResTextFile(name string) string {
	b, err := c.resourcesFS.ReadFile(name)
	if err != nil {
		panic(err)
	}
	return string(b)
}

// ServeResFile serves the content of a resources file as a response to a web request.
func (c *Connector) ServeResFile(name string, w http.ResponseWriter, r *http.Request) error {
	b, err := c.resourcesFS.ReadFile(name)
	if err != nil {
		return errors.New("", http.StatusNotFound)
	}
	hash := sha256.New()
	hash.Write(b)
	eTag := hex.EncodeToString(hash.Sum(nil))
	w.Header().Set("Etag", eTag)
	cacheControl := w.Header().Get("Cache-Control")
	if cacheControl == "" {
		w.Header().Set("Cache-Control", "max-age=3600, private, stale-while-revalidate=3600")
	}
	contentType := w.Header().Get("Content-Type")
	if contentType == "" {
		contentType = http.DetectContentType(b)
		w.Header().Set("Content-Type", contentType)
	}
	if r.Header.Get("If-None-Match") == eTag {
		w.WriteHeader(http.StatusNotModified)
		return nil
	}
	w.Header().Set("Content-Length", strconv.Itoa(len(b)))
	_, err = w.Write(b)
	if err != nil {
		return errors.Trace(err)
	}
	return nil
}

// ExecuteResTemplate parses the resource file as a template, executes it given the data, and returns the result.
// The template is assumed to be a text template unless the file name ends in .html, in which case it is processed as an HTML template.
//
// {{ var | attr }}, {{ var | url }}, {{ var | css }} or {{ var | safe }} may be used to prevent the escaping of a variable in an HTML template.
// These map to [htmltemplate.HTMLAttr], [htmltemplate.URL], [htmltemplate.CSS] and [htmltemplate.HTML] respectively.
// Use of these types presents a security risk.
func (c *Connector) ExecuteResTemplate(name string, data any) ([]byte, error) {
	templateFile, err := c.resourcesFS.ReadFile(name)
	if err != nil {
		return nil, errors.Trace(err)
	}
	var rendered bytes.Buffer
	if strings.HasSuffix(strings.ToLower(name), ".html") {
		funcMap := htmltemplate.FuncMap{
			"attr": func(s string) htmltemplate.HTMLAttr {
				return htmltemplate.HTMLAttr(s)
			},
			"url": func(s string) htmltemplate.URL {
				return htmltemplate.URL(s)
			},
			"css": func(s string) htmltemplate.CSS {
				return htmltemplate.CSS(s)
			},
			"safe": func(s string) htmltemplate.HTML {
				return htmltemplate.HTML(s)
			},
		}
		htmlTmpl, err := htmltemplate.New(name).
			Funcs(funcMap).
			Parse(utils.UnsafeBytesToString(templateFile))
		if err != nil {
			return nil, errors.Trace(err)
		}
		err = htmlTmpl.ExecuteTemplate(&rendered, name, data)
		if err != nil {
			return nil, errors.Trace(err)
		}
	} else {
		textTmpl, err := texttemplate.New(name).
			Parse(utils.UnsafeBytesToString(templateFile))
		if err != nil {
			return nil, errors.Trace(err)
		}
		err = textTmpl.ExecuteTemplate(&rendered, name, data)
		if err != nil {
			return nil, errors.Trace(err)
		}
	}
	return rendered.Bytes(), nil
}

// initStringBundle reads text.yaml from the FS into an in-memory map.
func (c *Connector) initStringBundle() error {
	c.stringBundle = nil
	b, err := c.ReadResFile("text.yaml")
	if errors.Is(err, os.ErrNotExist) {
		b, err = c.ReadResFile("strings.yaml") // Backward compatibility
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
	}
	if err != nil {
		return errors.Trace(err)
	}
	if len(b) == 0 {
		return nil
	}
	err = yaml.NewDecoder(bytes.NewReader(b)).Decode(&c.stringBundle)
	if err != nil {
		return errors.Trace(err)
	}
	return nil
}

/*
LoadResString returns a localized string from the string bundle best matched to the locale of the context.
The string bundle is a YAML file that must be loadable from the service's resource FS with the name text.yaml.
The YAML should map a string key to its localized values on a per language basis:

	Localized:
	  en: Localized
	  en-UK: Localised
	  fr: Localisée
	  it: Localizzato
	  default: Localized

If a default is not explicitly provided, English (en) is used as the fallback language.
String keys and ISO 639 language codes are case sensitive.
*/
func (c *Connector) LoadResString(ctx context.Context, stringKey string) (value string, err error) {
	if c.stringBundle == nil {
		return "", errors.New("string bundle text.yaml not found in resource FS")
	}
	txl := c.stringBundle[stringKey]
	if txl == nil {
		return "", errors.New("no string matches the key '%s'", stringKey)
	}
	// da, en-gb;q=0.8, en;q=0.7
	full := frame.Of(ctx).Header().Get("Accept-Language")
	var qMax float64
	segments := strings.Split(full, ",")
	var result string
	for _, seg := range segments {
		seg = strings.TrimSpace(seg)
		if seg == "" {
			continue
		}
		var q float64
		lang, after, found := strings.Cut(seg, ";")
		if !found {
			// da
			q = 1.0
		} else {
			// en-gb;q=0.8
			qStr := strings.TrimLeft(after, " q=")
			q, _ = strconv.ParseFloat(qStr, 64)
		}
		if q <= qMax {
			continue
		}
		for {
			v, ok := txl[lang]
			if ok {
				result = v
				qMax = q
				break
			}
			lang, _, found = strings.Cut(lang, "-")
			if !found {
				break
			}
		}
	}
	if qMax == 0 && result == "" {
		var ok bool
		result, ok = txl["default"]
		if !ok {
			result, ok = txl["en"]
		}
		if !ok {
			result = txl["en-US"]
		}
	}
	return result, nil
}

// MustLoadResString returns a string from the string bundle. It panics if the string is not found.
func (c *Connector) MustLoadResString(ctx context.Context, stringKey string) string {
	s, err := c.LoadResString(ctx, stringKey)
	if err != nil {
		panic(err)
	}
	return s
}

// LoadResStrings returns all strings from the string bundle best matched to the locale in the context.
func (c *Connector) LoadResStrings(ctx context.Context) (valuesByStringKey map[string]string, err error) {
	if c.stringBundle == nil {
		return nil, errors.New("string bundle text.yaml not found in resource FS")
	}
	valuesByStringKey = map[string]string{}
	for key := range c.stringBundle {
		valuesByStringKey[key], err = c.LoadResString(ctx, key)
		if err != nil {
			return nil, errors.Trace(err)
		}
	}
	return valuesByStringKey, nil
}

// MustLoadResStrings returns all strings from from the string bundle. It panics if the string bundle is not found.
func (c *Connector) MustLoadResStrings(ctx context.Context) (valuesByStringKey map[string]string) {
	m, err := c.LoadResStrings(ctx)
	if err != nil {
		panic(err)
	}
	return m
}
