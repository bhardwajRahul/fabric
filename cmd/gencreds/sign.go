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
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/nats-io/jwt/v2"
	"github.com/nats-io/nkeys"
)

// budgetErr signals that the rule set's resulting JWT is unlikely to fit the
// NATS server's default 4 KB max_control_line. The caller exits with code 2
// when this is the underlying error, so CI can distinguish it from arbitrary
// failures.
type budgetErr struct {
	hostname string
	bytes    int
	limit    int
}

func (e *budgetErr) Error() string {
	return fmt.Sprintf("%s: signed JWT is %d bytes, exceeds %d-byte CONNECT budget", e.hostname, e.bytes, e.limit)
}

// jwtBytesBudget is a sanity ceiling on the encoded JWT size. nats-server's
// default max_control_line is 4096 bytes; framing eats ~50 bytes; the
// remainder must hold the base64-encoded JWT. The aclBudgetSize check
// (3 KB raw permission JSON) runs first; this is the post-encoding
// belt-and-suspenders.
const jwtBytesBudget = 4000

// loadAccountKey reads an account NKey seed file (operator's signing key) and
// returns a KeyPair.
func loadAccountKey(path string) (nkeys.KeyPair, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	seed := bytes.TrimSpace(data)
	kp, err := nkeys.FromSeed(seed)
	if err != nil {
		return nil, fmt.Errorf("parse account seed: %w", err)
	}
	pub, err := kp.PublicKey()
	if err != nil {
		return nil, fmt.Errorf("read account public key: %w", err)
	}
	if !strings.HasPrefix(pub, "A") {
		return nil, fmt.Errorf("signing key is not an account NKey (public key: %s...)", pub[:1])
	}
	return kp, nil
}

// signService produces the .creds bytes for one service. It scans the
// service's source for ACL-relevant call patterns, builds the rule set,
// substitutes {{plane}}, and signs with the account key.
func signService(s service, accountKP nkeys.KeyPair, cfg config) ([]byte, error) {
	in, err := scanService(s.Dir, s.Dir)
	if err != nil {
		return nil, fmt.Errorf("scan source: %w", err)
	}
	rules, _, err := buildACLRules(in)
	if err != nil {
		return nil, fmt.Errorf("build rules: %w", err)
	}
	rules = subsumptionDedup(rules)
	if size := aclBudgetSize(rules); size > aclBytesBudget {
		return nil, fmt.Errorf("%s: rule set %d bytes exceeds %d-byte JWT permission budget (top: %s)",
			s.Hostname, size, aclBytesBudget, topPatterns(rules, 3))
	}
	pub, sub := splitRules(rules, cfg.plane)

	userKP, err := userKeyPair(s.Hostname, cfg)
	if err != nil {
		return nil, fmt.Errorf("user key: %w", err)
	}
	userPub, err := userKP.PublicKey()
	if err != nil {
		return nil, fmt.Errorf("user public key: %w", err)
	}
	userSeed, err := userKP.Seed()
	if err != nil {
		return nil, fmt.Errorf("user seed: %w", err)
	}

	claims := jwt.NewUserClaims(userPub)
	claims.Name = s.Hostname
	claims.Permissions.Pub.Allow = pub
	claims.Permissions.Sub.Allow = sub
	if cfg.expiration > 0 {
		claims.Expires = time.Now().Add(cfg.expiration).Unix()
	}

	signed, err := claims.Encode(accountKP)
	if err != nil {
		return nil, fmt.Errorf("encode jwt: %w", err)
	}
	if len(signed) > jwtBytesBudget {
		return nil, &budgetErr{hostname: s.Hostname, bytes: len(signed), limit: jwtBytesBudget}
	}

	creds, err := jwt.FormatUserConfig(signed, userSeed)
	if err != nil {
		return nil, fmt.Errorf("format creds: %w", err)
	}
	return creds, nil
}

// splitRules separates rules into PUB and SUB allow-lists with {{plane}}
// substituted. The separation is the shape jwt.UserClaims.Permissions
// expects; substitution pins each rule to the deploy's plane.
func splitRules(rules []aclRule, plane string) (pub, sub []string) {
	for _, r := range rules {
		subj := strings.ReplaceAll(r.Subject, "{{plane}}", plane)
		switch r.Verb {
		case "PUB":
			pub = append(pub, subj)
		case "SUB":
			sub = append(sub, subj)
		}
	}
	return pub, sub
}

// userKeyPair returns the per-service user NKey pair. With --persist-user-nkeys,
// it reads a stable key from <persist>/<hostname>_user.nk if present, or
// writes a fresh key there if absent. With --rotate (default), it generates a
// fresh key on every call.
func userKeyPair(hostname string, cfg config) (nkeys.KeyPair, error) {
	if cfg.persist != "" {
		if err := os.MkdirAll(cfg.persist, 0o700); err != nil {
			return nil, fmt.Errorf("create persist dir: %w", err)
		}
		path := filepath.Join(cfg.persist, hostname+"_user.nk")
		if data, err := os.ReadFile(path); err == nil {
			kp, err := nkeys.FromSeed(bytes.TrimSpace(data))
			if err != nil {
				return nil, fmt.Errorf("parse persisted seed for %s: %w", hostname, err)
			}
			return kp, nil
		} else if !os.IsNotExist(err) {
			return nil, fmt.Errorf("read persisted seed for %s: %w", hostname, err)
		}
		kp, err := nkeys.CreateUser()
		if err != nil {
			return nil, err
		}
		seed, err := kp.Seed()
		if err != nil {
			return nil, err
		}
		if err := os.WriteFile(path, seed, 0o600); err != nil {
			return nil, fmt.Errorf("write persisted seed for %s: %w", hostname, err)
		}
		return kp, nil
	}
	return nkeys.CreateUser()
}
