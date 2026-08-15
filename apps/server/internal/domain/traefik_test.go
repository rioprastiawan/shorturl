package domain

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/rioprastiawan/shorturl/apps/server/internal/config"
	"github.com/rioprastiawan/shorturl/apps/server/internal/store"
)

func TestSanitizeHostname(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "dots become dashes", input: "go.example.com", want: "go-example-com"},
		{name: "existing hyphens survive", input: "go-links.example.com", want: "go-links-example-com"},
		{name: "uppercase is folded", input: "GO.Example.COM", want: "go-example-com"},
		{name: "digits are kept", input: "go2.example.com", want: "go2-example-com"},
		{name: "deep subdomain", input: "a.b.c.example.com", want: "a-b-c-example-com"},
		{name: "trailing dot leaves no trailing dash", input: "go.example.com.", want: "go-example-com"},
		{name: "unexpected characters are replaced", input: "go_links@example.com", want: "go-links-example-com"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sanitizeHostname(tt.input)
			if got != tt.want {
				t.Errorf("sanitizeHostname(%q) = %q, want %q", tt.input, got, tt.want)
			}
			for _, r := range got {
				allowed := (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-'
				if !allowed {
					t.Errorf("sanitizeHostname(%q) = %q contains %q, which is not valid in a router name", tt.input, got, r)
				}
			}
		})
	}
}

func TestTraefikRouterYAML(t *testing.T) {
	got := traefikRouterYAML("go.example.com", "letsencrypt")

	want := "# Managed by ShortURL. Generated file - edits will be overwritten.\n" +
		"http:\n" +
		"  routers:\n" +
		"    shorturl-custom-go-example-com:\n" +
		"      rule: \"Host(`go.example.com`)\"\n" +
		"      entryPoints: [\"websecure\"]\n" +
		// The @docker suffix is load-bearing: without it Traefik resolves the
		// name inside the file provider and every custom domain 404s.
		"      service: shorturl-server@docker\n" +
		"      tls:\n" +
		"        certResolver: letsencrypt\n"

	if got != want {
		t.Errorf("traefikRouterYAML() =\n%s\nwant\n%s", got, want)
	}
}

func TestTraefikRouterYAMLUsesTheConfiguredResolver(t *testing.T) {
	got := traefikRouterYAML("link.example.org", "internal-ca")
	if !strings.Contains(got, "certResolver: internal-ca\n") {
		t.Errorf("router YAML = %q, want it to use the configured cert resolver", got)
	}
}

func TestTraefikFileName(t *testing.T) {
	if got, want := traefikFileName("go.example.com"), "shorturl-go-example-com.yml"; got != want {
		t.Errorf("traefikFileName() = %q, want %q", got, want)
	}
}

func TestSyncTraefikDirWritesOneFilePerDomain(t *testing.T) {
	dir := t.TempDir()

	if err := syncTraefikDir(dir, "letsencrypt", []string{"go.example.com", "link.example.org"}); err != nil {
		t.Fatalf("syncTraefikDir() error = %v", err)
	}

	assertDirContains(t, dir, "shorturl-go-example-com.yml", "shorturl-link-example-org.yml")

	content := readFile(t, filepath.Join(dir, "shorturl-go-example-com.yml"))
	if content != traefikRouterYAML("go.example.com", "letsencrypt") {
		t.Errorf("file content = %q, want the generated router", content)
	}
}

func TestSyncTraefikDirRemovesStaleManagedFilesButKeepsHandWrittenOnes(t *testing.T) {
	dir := t.TempDir()

	handWritten := filepath.Join(dir, "operator-override.yml")
	writeFile(t, handWritten, "http: {}\n")
	stale := filepath.Join(dir, "shorturl-old-example-com.yml")
	writeFile(t, stale, "stale\n")
	// A .tmp left behind by an interrupted write is ours to clean up.
	leftover := filepath.Join(dir, "shorturl-old-example-com.yml.tmp")
	writeFile(t, leftover, "half written")

	if err := syncTraefikDir(dir, "letsencrypt", []string{"go.example.com"}); err != nil {
		t.Fatalf("syncTraefikDir() error = %v", err)
	}

	assertDirContains(t, dir, "operator-override.yml", "shorturl-go-example-com.yml")

	if got := readFile(t, handWritten); got != "http: {}\n" {
		t.Errorf("hand-written file was rewritten: %q", got)
	}
}

func TestSyncTraefikDirRemovesEveryFileWhenNoDomainsAreActive(t *testing.T) {
	dir := t.TempDir()

	if err := syncTraefikDir(dir, "letsencrypt", []string{"go.example.com"}); err != nil {
		t.Fatalf("first sync error = %v", err)
	}
	if err := syncTraefikDir(dir, "letsencrypt", nil); err != nil {
		t.Fatalf("second sync error = %v", err)
	}

	assertDirContains(t, dir)
}

func TestSyncTraefikDirLeavesUnchangedFilesAlone(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "shorturl-go-example-com.yml")

	if err := syncTraefikDir(dir, "letsencrypt", []string{"go.example.com"}); err != nil {
		t.Fatalf("first sync error = %v", err)
	}
	before, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}

	if err := syncTraefikDir(dir, "letsencrypt", []string{"go.example.com"}); err != nil {
		t.Fatalf("second sync error = %v", err)
	}
	after, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}

	// Rewriting an identical file would make Traefik reload for nothing.
	if !before.ModTime().Equal(after.ModTime()) {
		t.Error("unchanged router file was rewritten")
	}
}

func TestSyncTraefikDirCreatesTheDirectory(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "dynamic")

	if err := syncTraefikDir(dir, "letsencrypt", []string{"go.example.com"}); err != nil {
		t.Fatalf("syncTraefikDir() error = %v", err)
	}
	assertDirContains(t, dir, "shorturl-go-example-com.yml")
}

func TestSyncTraefikDirReportsRouterNameCollisions(t *testing.T) {
	dir := t.TempDir()

	// Both sanitise to shorturl-go-example-com; one of them cannot be routed,
	// and the operator has to hear about it rather than lose traffic silently.
	err := syncTraefikDir(dir, "letsencrypt", []string{"go-example.com", "go.example.com"})
	if err == nil {
		t.Fatal("syncTraefikDir() error = nil, want a collision report")
	}
	if !strings.Contains(err.Error(), "go.example.com") {
		t.Errorf("error = %v, want it to name the losing hostname", err)
	}
	assertDirContains(t, dir, "shorturl-go-example-com.yml")
}

func TestSyncTraefikIsANoOpWithoutADynamicDirectory(t *testing.T) {
	svc, st := newTestService(t, fakeResolver{})
	st.domains[uuid.New()] = store.Domain{Hostname: testHostname, Status: StatusActive}

	if err := svc.SyncTraefik(context.Background()); err != nil {
		t.Fatalf("SyncTraefik() error = %v, want nil in development", err)
	}
}

func TestSyncTraefikWritesOnlyActiveDomains(t *testing.T) {
	dir := t.TempDir()
	st := newFakeStore()
	svc := NewService(st, nil, config.Config{
		AppDomain:           testAppDomain,
		TraefikDynamicDir:   dir,
		TraefikCertResolver: "letsencrypt",
	})

	active := uuid.New()
	st.domains[active] = store.Domain{ID: active, Hostname: "go.example.com", Status: StatusActive}
	pending := uuid.New()
	st.domains[pending] = store.Domain{ID: pending, Hostname: "pending.example.com", Status: StatusPending}

	if err := svc.SyncTraefik(context.Background()); err != nil {
		t.Fatalf("SyncTraefik() error = %v", err)
	}
	assertDirContains(t, dir, "shorturl-go-example-com.yml")
}

func assertDirContains(t *testing.T, dir string, want ...string) {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read dir: %v", err)
	}
	var got []string
	for _, entry := range entries {
		got = append(got, entry.Name())
	}
	slices.Sort(got)
	slices.Sort(want)
	if !slices.Equal(got, want) {
		t.Errorf("directory contains %v, want %v", got, want)
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(b)
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
