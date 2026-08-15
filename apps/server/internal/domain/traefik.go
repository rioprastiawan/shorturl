package domain

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/rioprastiawan/shorturl/apps/server/internal/store"
)

const (
	// managedPrefix marks the files this package owns. Anything in the dynamic
	// directory without it was written by an operator and is never touched.
	managedPrefix = "shorturl-"
	fileExtension = ".yml"
	tmpExtension  = ".tmp"
)

// SyncTraefik makes the Traefik dynamic directory match the set of active
// domains: one router file per active domain, and nothing left over.
//
// This is what lets a verified domain start serving HTTPS without editing
// docker-compose.yml or restarting a container (plan §7, §21). Traefik watches
// the directory and reloads on change; the certificate is requested only once a
// router exists, which is why unverified domains never burn ACME rate limit.
//
// Safe to call at any time, including at startup to repair a directory that
// drifted while the process was down.
func (s *Service) SyncTraefik(ctx context.Context) error {
	dir := s.cfg.TraefikDynamicDir
	if dir == "" {
		// Development: there is no Traefik to configure.
		return nil
	}

	domains, err := s.q.ListActiveDomains(ctx)
	if err != nil {
		return fmt.Errorf("list active domains: %w", err)
	}

	return syncTraefikDir(dir, s.cfg.TraefikCertResolver, activeHostnames(domains))
}

// syncTraefikDir is the filesystem half of SyncTraefik, split out so it can be
// tested against a temporary directory.
func syncTraefikDir(dir, certResolver string, hostnames []string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create %s: %w", dir, err)
	}

	var errs []error

	// Sort so a name collision resolves the same way on every run rather than
	// flapping between two hostnames.
	sorted := slices.Sorted(slices.Values(hostnames))

	wanted := make(map[string]string, len(sorted))
	owner := make(map[string]string, len(sorted))
	for _, hostname := range sorted {
		name := traefikFileName(hostname)
		if previous, taken := owner[name]; taken {
			// Two hostnames can sanitise to the same router name, for example
			// go.example.com and go-example.com. Keep the first and report the
			// second rather than silently overwriting its route.
			errs = append(errs, fmt.Errorf(
				"traefik router name %q is claimed by both %s and %s; %s has no route",
				name, previous, hostname, hostname))
			continue
		}
		owner[name] = hostname
		wanted[name] = traefikRouterYAML(hostname, certResolver)
	}

	for name, content := range wanted {
		if err := writeFileAtomic(filepath.Join(dir, name), content); err != nil {
			errs = append(errs, err)
		}
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		errs = append(errs, fmt.Errorf("read %s: %w", dir, err))
		return errors.Join(errs...)
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !isManagedFile(name) {
			continue
		}
		if _, keep := wanted[name]; keep {
			continue
		}
		if err := os.Remove(filepath.Join(dir, name)); err != nil && !errors.Is(err, fs.ErrNotExist) {
			errs = append(errs, fmt.Errorf("remove %s: %w", name, err))
		}
	}

	return errors.Join(errs...)
}

// isManagedFile reports whether this package wrote the file, and may therefore
// delete it. Leftover .tmp files from an interrupted write are included so they
// do not accumulate.
func isManagedFile(name string) bool {
	if !strings.HasPrefix(name, managedPrefix) {
		return false
	}
	return strings.HasSuffix(name, fileExtension) || strings.HasSuffix(name, fileExtension+tmpExtension)
}

func traefikFileName(hostname string) string {
	return managedPrefix + sanitizeHostname(hostname) + fileExtension
}

func traefikRouterName(hostname string) string {
	return managedPrefix + "custom-" + sanitizeHostname(hostname)
}

// backendService is the server's load-balancer service, as Traefik names it
// from the container's Docker labels.
//
// The "@docker" suffix is required and not cosmetic. Traefik resolves an
// unqualified service name inside the *same* provider, so a router written by
// the file provider would look for "shorturl-server@file", find nothing, and
// serve 404 for every custom domain while logging only a service-not-found
// error. Cross-provider references must name the provider.
const backendService = "shorturl-server@docker"

// traefikRouterYAML renders one router. It carries no loadBalancer block
// because the backend is already defined by the server container's labels.
func traefikRouterYAML(hostname, certResolver string) string {
	var b strings.Builder
	b.WriteString("# Managed by ShortURL. Generated file - edits will be overwritten.\n")
	b.WriteString("http:\n")
	b.WriteString("  routers:\n")
	fmt.Fprintf(&b, "    %s:\n", traefikRouterName(hostname))
	fmt.Fprintf(&b, "      rule: \"Host(`%s`)\"\n", hostname)
	b.WriteString("      entryPoints: [\"websecure\"]\n")
	fmt.Fprintf(&b, "      service: %s\n", backendService)
	b.WriteString("      tls:\n")
	fmt.Fprintf(&b, "        certResolver: %s\n", certResolver)
	return b.String()
}

// sanitizeHostname reduces a hostname to the characters a Traefik router name
// and a filename can both carry.
func sanitizeHostname(hostname string) string {
	var b strings.Builder
	b.Grow(len(hostname))
	for _, r := range strings.ToLower(hostname) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
		default:
			b.WriteByte('-')
		}
	}
	return strings.Trim(b.String(), "-")
}

// writeFileAtomic writes through a temporary file so Traefik, which reloads on
// any change it notices, never parses a half-written router.
//
// An unchanged file is left alone: rewriting it would trigger a reload of the
// whole dynamic configuration for nothing.
func writeFileAtomic(path, content string) error {
	if existing, err := os.ReadFile(path); err == nil && string(existing) == content {
		return nil
	}

	tmp := path + tmpExtension
	if err := os.WriteFile(tmp, []byte(content), 0o644); err != nil {
		return fmt.Errorf("write %s: %w", tmp, err)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("rename %s: %w", tmp, err)
	}
	return nil
}

// activeHostnames filters defensively: a router file must never exist for a
// domain that is not active, whatever the query returned.
func activeHostnames(domains []store.Domain) []string {
	hostnames := make([]string, 0, len(domains))
	for _, d := range domains {
		if d.Status == StatusActive {
			hostnames = append(hostnames, d.Hostname)
		}
	}
	return hostnames
}
