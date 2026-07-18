// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

package git

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os/exec"
	"runtime"
	"strings"
)

// BrowseURL prefers the upstream remote's forge homepage, then origin.
func BrowseURL(ctx context.Context, r Runner) (string, error) {
	remote := "origin"
	if hasRemote(ctx, r, "upstream") {
		remote = "upstream"
	} else if !hasRemote(ctx, r, "origin") {
		return "", errors.New("no upstream or origin remote to browse")
	}
	return BrowseURLForRemote(ctx, r, remote)
}

// BrowseURLForRemote is BrowseURL but for a specific remote.
func BrowseURLForRemote(ctx context.Context, r Runner, remote string) (string, error) {
	raw, err := RemoteURL(ctx, r, remote)
	if err != nil {
		return "", err
	}
	return HTTPBrowseURL(raw)
}

// HTTPBrowseURL converts a git remote URL into an https page URL.
func HTTPBrowseURL(remote string) (string, error) {
	remote = strings.TrimSpace(remote)
	remote = strings.TrimSuffix(remote, ".git")

	// git@host:owner/repo
	if strings.HasPrefix(remote, "git@") {
		rest := strings.TrimPrefix(remote, "git@")
		host, path, ok := strings.Cut(rest, ":")
		if !ok || path == "" {
			return "", fmt.Errorf("unrecognized SSH remote URL %q", remote)
		}
		return "https://" + host + "/" + path, nil
	}

	// ssh://git@host/owner/repo
	if strings.HasPrefix(remote, "ssh://") {
		u, err := url.Parse(remote)
		if err != nil {
			return "", err
		}
		path := strings.TrimPrefix(u.Path, "/")
		return "https://" + u.Hostname() + "/" + path, nil
	}

	// https://host/owner/repo
	if strings.HasPrefix(remote, "http://") || strings.HasPrefix(remote, "https://") {
		u, err := url.Parse(remote)
		if err != nil {
			return "", err
		}
		// Drop credentials if any.
		u.User = nil
		return strings.TrimSuffix(u.String(), ".git"), nil
	}

	return "", fmt.Errorf("unrecognized remote URL %q", remote)
}

// OpenBrowser opens url with the platform default browser.
func OpenBrowser(rawURL string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", rawURL)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", rawURL)
	default:
		cmd = exec.Command("xdg-open", rawURL)
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("open browser: %w", err)
	}
	return nil
}

// Browse opens the preferred forge homepage in a browser.
func Browse(ctx context.Context, r Runner) (string, error) {
	u, err := BrowseURL(ctx, r)
	if err != nil {
		return "", err
	}
	if err := OpenBrowser(u); err != nil {
		return u, err
	}
	return u, nil
}
