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

import "strings"

// subsumptionDedup drops any rule whose subject pattern is strictly
// subsumed by another rule in the same set (same verb). Lossless; NATS
// treats a matching ACL line as authorization, so removing a rule that's
// already covered by a broader sibling changes nothing on the wire.
func subsumptionDedup(rules []aclRule) []aclRule {
	pub, sub := splitByVerb(rules)
	pub = dedupVerb(pub)
	sub = dedupVerb(sub)
	out := make([]aclRule, 0, len(pub)+len(sub))
	for _, r := range pub {
		out = append(out, aclRule{Verb: "PUB", Subject: r})
	}
	for _, r := range sub {
		out = append(out, aclRule{Verb: "SUB", Subject: r})
	}
	return out
}

func splitByVerb(rules []aclRule) (pub, sub []string) {
	for _, r := range rules {
		switch r.Verb {
		case "PUB":
			pub = append(pub, r.Subject)
		case "SUB":
			sub = append(sub, r.Subject)
		}
	}
	return pub, sub
}

func dedupVerb(subjects []string) []string {
	dropped := make(map[int]bool, len(subjects))
	for i := range subjects {
		if dropped[i] {
			continue
		}
		for j := range subjects {
			if i == j || dropped[j] {
				continue
			}
			if subsumes(subjects[i], subjects[j]) {
				dropped[j] = true
			}
		}
	}
	out := make([]string, 0, len(subjects))
	for i, s := range subjects {
		if dropped[i] {
			continue
		}
		out = append(out, s)
	}
	return out
}

// subsumes reports whether pattern A is broad enough to cover pattern B.
// Equal patterns are not considered subsumption (so duplicates aren't
// erased to nothing).
func subsumes(a, b string) bool {
	if a == b {
		return false
	}
	aSegs := strings.Split(a, ".")
	bSegs := strings.Split(b, ".")
	for i, aSeg := range aSegs {
		if aSeg == ">" {
			return i < len(bSegs)
		}
		if i >= len(bSegs) {
			return false
		}
		bSeg := bSegs[i]
		switch {
		case aSeg == bSeg:
		case aSeg == "*":
		default:
			return false
		}
	}
	return len(aSegs) == len(bSegs)
}
