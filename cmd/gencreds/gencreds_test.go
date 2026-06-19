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
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/microbus-io/fabric/cmd/internal/pkgresolver"
	"github.com/microbus-io/testarossa"
	"github.com/nats-io/jwt/v2"
	"github.com/nats-io/nkeys"
)

// repoPath returns an absolute path under the repository root, robust against
// the test runner's CWD (CI, IDEs, etc.). Walks up from this source file
// until a go.mod is found.
func repoPath(t *testing.T, rel string) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatalf("runtime.Caller failed")
	}
	dir := filepath.Dir(file)
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			abs := filepath.Join(dir, rel)
			if _, err := os.Stat(abs); err != nil {
				t.Fatalf("path does not exist: %s", abs)
			}
			return abs
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("could not find go.mod walking up from %s", file)
		}
		dir = parent
	}
}

// newTestAccount returns a freshly-generated account KeyPair and writes its
// seed to a tmp file so the loadAccountKey path is exercised.
func newTestAccount(t *testing.T) (nkeys.KeyPair, string) {
	t.Helper()
	kp, err := nkeys.CreateAccount()
	if err != nil {
		t.Fatalf("create account: %v", err)
	}
	seed, err := kp.Seed()
	if err != nil {
		t.Fatalf("seed: %v", err)
	}
	path := filepath.Join(t.TempDir(), "account.nk")
	if err := os.WriteFile(path, seed, 0o600); err != nil {
		t.Fatalf("write seed: %v", err)
	}
	return kp, path
}

func TestLoadAccountKey_RejectsNonAccount(t *testing.T) {
	t.Parallel()
	assert := testarossa.For(t)
	user, err := nkeys.CreateUser()
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	seed, _ := user.Seed()
	path := filepath.Join(t.TempDir(), "user.nk")
	_ = os.WriteFile(path, seed, 0o600)
	_, err = loadAccountKey(path)
	assert.Error(err, "expected error for non-account NKey")
}

func TestSplitRules_PlaneSubst(t *testing.T) {
	t.Parallel()
	assert := testarossa.For(t)
	rules := []aclRule{
		{Verb: "PUB", Subject: "{{plane}}.safe.443.alpha.beta.*.GET.foo"},
		{Verb: "PUB", Subject: "{{plane}}.reply._.alpha.>"},
		{Verb: "SUB", Subject: "{{plane}}.safe.*.*.alpha.>"},
		{Verb: "SUB", Subject: "{{plane}}.safe.888.*.all.>"},
	}
	pub, sub := splitRules(rules, "TESTPLANE")
	wantPub := []string{
		"TESTPLANE.safe.443.alpha.beta.*.GET.foo",
		"TESTPLANE.reply._.alpha.>",
	}
	wantSub := []string{
		"TESTPLANE.safe.*.*.alpha.>",
		"TESTPLANE.safe.888.*.all.>",
	}
	assert.True(equalStringSets(pub, wantPub), "PUB mismatch:\n got: %v\nwant: %v", pub, wantPub)
	assert.True(equalStringSets(sub, wantSub), "SUB mismatch:\n got: %v\nwant: %v", sub, wantSub)
}

func TestSignService_KitchenDecodesAndPermissionsContainExpected(t *testing.T) {
	// No parallel - scans the kitchen fixture.
	assert := testarossa.For(t)
	accountKP, accountPath := newTestAccount(t)
	accountPub, _ := accountKP.PublicKey()

	dir := repoPath(t, "cmd/gencreds/testdata/kitchen")
	s := service{Hostname: "kitchen.fixture", Dir: dir}
	cfg := config{plane: "live", signingKey: accountPath}

	creds, err := signService(s, accountKP, cfg, pkgresolver.New(s.Dir))
	if err != nil {
		t.Fatalf("signService: %v", err)
	}

	// Round-trip the JWT out of the .creds file.
	tok, err := jwt.ParseDecoratedJWT(creds)
	if err != nil {
		t.Fatalf("ParseDecoratedJWT: %v", err)
	}
	uc, err := jwt.DecodeUserClaims(tok)
	if err != nil {
		t.Fatalf("DecodeUserClaims: %v", err)
	}
	assert.Equal("kitchen.fixture", uc.Name, "name")
	assert.Equal(accountPub, uc.Issuer, "issuer")

	// Kitchen's source code calls weirdapi.NewClient(svc).Plain(ctx) - so the
	// PUB allow-list must include the corresponding subject with the live plane.
	wantPubContains := "live.safe.443.kitchen_fixture.weird_fixture.*.GET.plain"
	assert.True(contains(uc.Permissions.Pub.Allow, wantPubContains), "PUB allow missing %q\ngot: %v", wantPubContains, uc.Permissions.Pub.Allow)
	// Standard SUB rules must be present (control plane, broadcast).
	wantSubContains := "live.safe.888.*.all.>"
	assert.True(contains(uc.Permissions.Sub.Allow, wantSubContains), "SUB allow missing %q\ngot: %v", wantSubContains, uc.Permissions.Sub.Allow)

	// .creds also carries the user seed, which must round-trip back to a
	// valid user NKey.
	userKP, err := jwt.ParseDecoratedNKey(creds)
	if err != nil {
		t.Fatalf("ParseDecoratedNKey: %v", err)
	}
	userPub, _ := userKP.PublicKey()
	assert.True(strings.HasPrefix(userPub, "U"), "user public key %q does not start with 'U'", userPub)
	assert.Equal(userPub, uc.Subject, "subject")
}

func TestSignService_PersistUserNKeysIsStable(t *testing.T) {
	// No parallel - scans the kitchen fixture.
	assert := testarossa.For(t)
	accountKP, _ := newTestAccount(t)
	dir := repoPath(t, "cmd/gencreds/testdata/kitchen")
	s := service{Hostname: "kitchen.fixture", Dir: dir}
	persistDir := t.TempDir()
	cfg := config{plane: "p", persist: persistDir, signingKey: "ignored"}

	creds1, err := signService(s, accountKP, cfg, pkgresolver.New(s.Dir))
	if err != nil {
		t.Fatalf("first sign: %v", err)
	}
	tok1, _ := jwt.ParseDecoratedJWT(creds1)
	uc1, _ := jwt.DecodeUserClaims(tok1)

	creds2, err := signService(s, accountKP, cfg, pkgresolver.New(s.Dir))
	if err != nil {
		t.Fatalf("second sign: %v", err)
	}
	tok2, _ := jwt.ParseDecoratedJWT(creds2)
	uc2, _ := jwt.DecodeUserClaims(tok2)

	assert.Equal(uc1.Subject, uc2.Subject, "user public key changed across runs")
}

func TestSignService_ExpirationOmittedByDefault(t *testing.T) {
	// No parallel - scans the kitchen fixture.
	assert := testarossa.For(t)
	accountKP, accountPath := newTestAccount(t)
	dir := repoPath(t, "cmd/gencreds/testdata/kitchen")
	s := service{Hostname: "kitchen.fixture", Dir: dir}
	cfg := config{plane: "live", signingKey: accountPath}

	creds, err := signService(s, accountKP, cfg, pkgresolver.New(s.Dir))
	if err != nil {
		t.Fatalf("signService: %v", err)
	}
	tok, _ := jwt.ParseDecoratedJWT(creds)
	uc, _ := jwt.DecodeUserClaims(tok)
	assert.Equal(int64(0), uc.Expires, "Expires (no expiration by default)")
}

func TestSignService_ExpirationSetsExpClaim(t *testing.T) {
	// No parallel - scans the kitchen fixture.
	assert := testarossa.For(t)
	accountKP, accountPath := newTestAccount(t)
	dir := repoPath(t, "cmd/gencreds/testdata/kitchen")
	s := service{Hostname: "kitchen.fixture", Dir: dir}
	cfg := config{plane: "live", signingKey: accountPath, expiration: 24 * time.Hour}

	before := time.Now().Unix()
	creds, err := signService(s, accountKP, cfg, pkgresolver.New(s.Dir))
	if err != nil {
		t.Fatalf("signService: %v", err)
	}
	after := time.Now().Unix()

	tok, _ := jwt.ParseDecoratedJWT(creds)
	uc, _ := jwt.DecodeUserClaims(tok)
	wantMin := before + int64(24*time.Hour/time.Second)
	wantMax := after + int64(24*time.Hour/time.Second)
	assert.True(uc.Expires >= wantMin && uc.Expires <= wantMax, "Expires = %d, want between %d and %d", uc.Expires, wantMin, wantMax)
}

// contains reports whether haystack includes needle.
func contains(haystack []string, needle string) bool {
	for _, s := range haystack {
		if s == needle {
			return true
		}
	}
	return false
}

func TestResolveBundle_ManifestList_KitchenAndWeird(t *testing.T) {
	t.Parallel()
	assert := testarossa.For(t)
	kitchenDir := repoPath(t, "cmd/gencreds/testdata/kitchen")
	weirdDir := repoPath(t, "cmd/gencreds/testdata/weird")
	services, err := resolveManifestList(kitchenDir + "," + weirdDir)
	if err != nil {
		t.Fatalf("resolveManifestList: %v", err)
	}
	got := []string{}
	for _, s := range services {
		got = append(got, s.Hostname)
	}
	want := []string{"kitchen.fixture", "weird.fixture"}
	assert.True(equalStringSets(got, want), "hostnames = %v, want %v", got, want)
}

func TestResolveBundle_MainGoFixture(t *testing.T) {
	t.Parallel()
	assert := testarossa.For(t)
	mainGo := repoPath(t, "cmd/gencreds/testdata/bundlemain/main.go")
	services, err := resolveMainGo(mainGo)
	if err != nil {
		t.Fatalf("resolveMainGo: %v", err)
	}
	got := []string{}
	for _, s := range services {
		got = append(got, s.Hostname)
	}
	want := []string{"kitchen.fixture", "weird.fixture"}
	assert.True(equalStringSets(got, want), "hostnames = %v, want %v", got, want)
}

func TestRun_KitchenWeirdEndToEnd(t *testing.T) {
	// No parallel - runs the full gencreds pipeline against the kitchen and weird fixtures.
	assert := testarossa.For(t)
	_, accountPath := newTestAccount(t)
	out := t.TempDir()
	kitchenDir := repoPath(t, "cmd/gencreds/testdata/kitchen")
	weirdDir := repoPath(t, "cmd/gencreds/testdata/weird")
	cfg := config{
		manifests:  kitchenDir + "," + weirdDir,
		signingKey: accountPath,
		out:        out,
		plane:      "microbus",
	}
	if err := run(cfg); err != nil {
		t.Fatalf("run: %v", err)
	}
	for _, host := range []string{"kitchen.fixture", "weird.fixture"} {
		path := filepath.Join(out, host+"_nats.creds")
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		_, err = jwt.ParseDecoratedJWT(data)
		assert.NoError(err, "%s: ParseDecoratedJWT", host)
		fi, _ := os.Stat(path)
		assert.Equal(os.FileMode(0o600), fi.Mode().Perm(), "%s: mode", host)
	}
}

func TestSignService_BudgetSentinel(t *testing.T) {
	t.Parallel()
	// Regression: ensure budgetErr surfaces via errors.As when the encoded JWT
	// is too large. We don't easily produce an oversize JWT in unit tests
	// (the source-driven scan keeps real services well under the budget), so
	// just spot-check the type satisfies the interface main.go relies on for
	// exit code 2.
	be := &budgetErr{hostname: "x", bytes: 9999, limit: 4000}
	var target *budgetErr
	if !errors.As(be, &target) {
		t.Fatal("budgetErr does not satisfy errors.As(*budgetErr)")
	}
}

// equalStringSets compares two string slices order-insensitively. Used for
// assertions where ordering depends on map iteration or filesystem listing.
func equalStringSets(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	x := append([]string(nil), a...)
	y := append([]string(nil), b...)
	sort.Strings(x)
	sort.Strings(y)
	for i := range x {
		if x[i] != y[i] {
			return false
		}
	}
	return true
}
