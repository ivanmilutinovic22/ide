package terminal

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/x/term"
)

// QueryBackgroundColor sends an OSC 11 query to the terminal and returns the
// background color as "#rrggbb". Returns "" if the terminal doesn't respond or
// the response can't be parsed. Must be called before bubbletea starts (while
// stdin/stdout are in normal state).
func QueryBackgroundColor() string {
	tty, err := os.OpenFile("/dev/tty", os.O_RDWR, 0)
	if err != nil {
		return ""
	}
	defer tty.Close()

	fd := tty.Fd()
	oldState, err := term.MakeRaw(fd)
	if err != nil {
		return ""
	}
	defer term.Restore(fd, oldState)

	// OSC 11 query: ask for background color. ST terminator works in all
	// modern terminals; BEL (\x07) is the fallback but ST (\x1b\\) is preferred.
	fmt.Fprint(tty, "\x1b]11;?\x1b\\")

	// Read response with a 200ms deadline.
	done := make(chan string, 1)
	go func() {
		buf := make([]byte, 64)
		n, err := tty.Read(buf)
		if err != nil || n == 0 {
			done <- ""
			return
		}
		done <- string(buf[:n])
	}()

	var raw string
	select {
	case raw = <-done:
	case <-time.After(200 * time.Millisecond):
		return ""
	}

	return parseOSC11(raw)
}

// parseOSC11 parses an OSC 11 response like:
//
//	\x1b]11;rgb:RRRR/GGGG/BBBB\x1b\\ or \x1b]11;rgb:RR/GG/BB\x07
//
// and returns "#rrggbb". The 16-bit channel values are down-sampled to 8-bit
// by taking the high byte.
func parseOSC11(s string) string {
	// Find the rgb: payload between "11;" and the ST/BEL terminator.
	idx := strings.Index(s, "rgb:")
	if idx == -1 {
		return ""
	}
	rgb := s[idx+4:]
	// Trim trailing ST (\x1b\) or BEL (\x07) and anything after.
	if i := strings.IndexAny(rgb, "\x1b\x07"); i != -1 {
		rgb = rgb[:i]
	}

	parts := strings.Split(rgb, "/")
	if len(parts) != 3 {
		return ""
	}

	var channels [3]int
	for i, p := range parts {
		p = strings.TrimSpace(p)
		if len(p) == 0 {
			return ""
		}
		// Terminals return 2 or 4 hex digits per channel.
		// For 4 digits (16-bit), take the high byte (first 2 digits).
		hex := p
		if len(hex) == 4 {
			hex = hex[:2]
		}
		var v int
		if _, err := fmt.Sscanf(hex, "%x", &v); err != nil {
			return ""
		}
		channels[i] = v
	}

	return fmt.Sprintf("#%02x%02x%02x", channels[0], channels[1], channels[2])
}
