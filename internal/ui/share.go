package ui

import (
	"encoding/base64"
	"fmt"
	"net/url"
	"strings"
)

// ShareBase is the zerebos/ghostty-config web app; its import page reads a
// URL-safe base64 config from the #share= fragment.
const ShareBase = "https://ghostty.zerebos.com/app/import-export"

// MaxShareURL matches the web app's limit for shareable links.
const MaxShareURL = 1800

// shareText is the staged config as plain key = value lines (no comments).
func (m Model) shareText() string {
	var sb strings.Builder
	for _, st := range m.sch.Settings {
		for _, v := range m.vals[st.Key] {
			sb.WriteString(st.Key + " = " + v + "\n")
		}
	}
	return sb.String()
}

// ShareURL returns a link that opens this config in the web app. ok is false
// when the link would exceed MaxShareURL.
func (m Model) ShareURL() (string, bool) {
	u := ShareBase + "#share=" + base64.RawURLEncoding.EncodeToString([]byte(m.shareText()))
	return u, len(u) <= MaxShareURL
}

// DecodeShare extracts the config text from a share link.
func DecodeShare(link string) (string, error) {
	_, frag, ok := strings.Cut(strings.TrimSpace(link), "#")
	if !ok {
		return "", fmt.Errorf("no #share= fragment")
	}
	q, err := url.ParseQuery(frag)
	if err != nil || q.Get("share") == "" {
		return "", fmt.Errorf("no #share= fragment")
	}
	b, err := base64.RawURLEncoding.DecodeString(strings.TrimRight(q.Get("share"), "="))
	if err != nil {
		return "", fmt.Errorf("bad share payload: %w", err)
	}
	return string(b), nil
}
