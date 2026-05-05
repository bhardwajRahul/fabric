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
	"sort"
	"strconv"
)

// aclBytesBudget is the per-service raw-permission-JSON budget (3 KB).
// Leaves ~1 KB for the JWT envelope (header, signature, standard claims)
// so the resulting CONNECT command fits in nats-server's default 4 KB
// max_control_line. Distinct from sign.go's `jwtBytesBudget`, which checks
// the *signed* JWT byte size after encoding.
const aclBytesBudget = 3000

// aclBudgetSize estimates the raw permission-JSON size that a rule set
// would produce inside a NATS user JWT. Models the `nats.Permissions`
// JSON shape that gets emitted at sign time:
// `{"pub":{"allow":[...]},"sub":{"allow":[...]}}`.
func aclBudgetSize(rules []aclRule) int {
	pubLen, pubCount := sumLengths(rules, "PUB")
	subLen, subCount := sumLengths(rules, "SUB")

	size := 0
	size += len(`{"pub":{"allow":[`)
	size += pubLen
	size += pubCount * 2 // surrounding quotes per element
	if pubCount > 0 {
		size += pubCount - 1 // commas
	}
	size += len(`]},"sub":{"allow":[`)
	size += subLen
	size += subCount * 2
	if subCount > 0 {
		size += subCount - 1
	}
	size += len(`]}}`)
	return size
}

func sumLengths(rules []aclRule, verb string) (int, int) {
	total, count := 0, 0
	for _, r := range rules {
		if r.Verb != verb {
			continue
		}
		total += len(r.Subject)
		count++
	}
	return total, count
}

// topPatterns returns the n longest subject patterns as a comma-joined
// string with each pattern's length annotated. Used in the budget-
// exceeded error message to point operators at the worst offenders.
func topPatterns(rules []aclRule, n int) string {
	sorted := append([]aclRule(nil), rules...)
	sort.SliceStable(sorted, func(i, j int) bool {
		return len(sorted[i].Subject) > len(sorted[j].Subject)
	})
	if n > len(sorted) {
		n = len(sorted)
	}
	out := ""
	for i := 0; i < n; i++ {
		if i > 0 {
			out += ", "
		}
		out += sorted[i].Verb + " " + sorted[i].Subject + " (" +
			strconv.Itoa(len(sorted[i].Subject)) + " B)"
	}
	return out
}
