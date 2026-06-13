package ui

import (
	"io"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/vt"
	"github.com/creack/pty"
)

// stripped returns the plain text of a vt.Emulator.Render() so we can inspect
// what would actually appear on screen, ignoring ANSI styling.
func renderPlain(em *vt.Emulator) string {
	return ansi.Strip(em.Render())
}

// rowAt returns the n-th line (0-based) of the plain rendered output, trimmed
// of trailing spaces.
func rowAt(out string, n int) string {
	lines := strings.Split(out, "\n")
	if n < 0 || n >= len(lines) {
		return ""
	}
	return strings.TrimRight(lines[n], " ")
}

func TestVTPlainTextAppearsAtTopLeft(t *testing.T) {
	em := vt.NewEmulator(20, 5)
	_, _ = em.Write([]byte("hello"))

	out := renderPlain(em)
	if got := rowAt(out, 0); got != "hello" {
		t.Fatalf("row 0 = %q, want %q", got, "hello")
	}
}

func TestVTNewlineAdvancesRow(t *testing.T) {
	em := vt.NewEmulator(20, 5)
	// LF without CR should reset column to 0 when LNM is enabled, mimicking
	// how capturePaneCmd feeds tmux capture-pane output.
	_, _ = em.Write([]byte("\x1b[20h"))
	_, _ = em.Write([]byte("row0\nrow1\nrow2"))

	out := renderPlain(em)
	if got := rowAt(out, 0); got != "row0" {
		t.Errorf("row 0 = %q, want %q", got, "row0")
	}
	if got := rowAt(out, 1); got != "row1" {
		t.Errorf("row 1 = %q, want %q", got, "row1")
	}
	if got := rowAt(out, 2); got != "row2" {
		t.Errorf("row 2 = %q, want %q", got, "row2")
	}
}

func TestVTTrailingNewlineScrollsTopAway(t *testing.T) {
	// Regression for the preview-top-trim bug: writing one \n past the last
	// row scrolls the buffer up and drops row 0.
	em := vt.NewEmulator(20, 3)
	_, _ = em.Write([]byte("\x1b[20h"))
	_, _ = em.Write([]byte("row0\nrow1\nrow2\n"))

	out := renderPlain(em)
	if got := rowAt(out, 0); got == "row0" {
		t.Logf("emulator did not scroll on trailing LF — fix in capturePaneCmd is defensive but unnecessary")
	}
	t.Logf("row0=%q row1=%q row2=%q", rowAt(out, 0), rowAt(out, 1), rowAt(out, 2))
}

func TestVTAltScreenSwitchPreservesNewWrites(t *testing.T) {
	em := vt.NewEmulator(20, 5)
	// Enter alt screen (what tmux client does immediately after attach).
	_, _ = em.Write([]byte("\x1b[?1049h"))
	// Clear + home (also typical).
	_, _ = em.Write([]byte("\x1b[2J\x1b[H"))
	_, _ = em.Write([]byte("attached"))

	out := renderPlain(em)
	if got := rowAt(out, 0); got != "attached" {
		t.Fatalf("alt screen row 0 = %q, want %q (em.Render full output = %q)", got, "attached", em.Render())
	}
}

func TestVTCursorPositioningWrites(t *testing.T) {
	em := vt.NewEmulator(20, 5)
	// Move to row 3 col 1 (CSI uses 1-based) and write.
	_, _ = em.Write([]byte("\x1b[3;1Hmid-screen"))

	out := renderPlain(em)
	if got := rowAt(out, 2); got != "mid-screen" {
		t.Fatalf("row 2 = %q, want %q", got, "mid-screen")
	}
}

// TestVTTmuxLikeBootSequence simulates the first burst of bytes a fresh tmux
// attach-session client sends: alt screen enter, clear, draw status line,
// draw window content. If the new vt library is the source of the
// "completely black PTY" bug, this test will produce empty output where we
// expect content.
func TestVTTmuxLikeBootSequence(t *testing.T) {
	em := vt.NewEmulator(80, 24)

	// Boot sequence approximating what tmux writes on attach.
	boot := strings.Join([]string{
		"\x1b[?1049h", // alt screen on
		"\x1b[?25l",   // hide cursor
		"\x1b[2J",     // clear screen
		"\x1b[H",      // cursor home
		"\x1b[1;1H",   // cursor (1,1)
		"hello from tmux",
		"\x1b[24;1H",                     // cursor to status row
		"\x1b[7m[0] window-name\x1b[27m", // reverse-video status
		"\x1b[?25h",                      // show cursor
	}, "")
	_, _ = em.Write([]byte(boot))

	out := renderPlain(em)
	if got := rowAt(out, 0); !strings.Contains(got, "hello from tmux") {
		t.Errorf("row 0 = %q, expected to contain 'hello from tmux'", got)
	}
	if got := rowAt(out, 23); !strings.Contains(got, "window-name") {
		t.Errorf("row 23 = %q, expected to contain 'window-name'", got)
	}
}

// TestVTPTYIntegration spawns a real child process attached to a PTY, pipes
// its stdout into a vt.Emulator (the same flow as our embedded terminal),
// and verifies that the rendered output contains the printed text. If this
// fails, the bug is upstream of our own code.
func TestVTPTYIntegration(t *testing.T) {
	em := vt.NewEmulator(80, 24)

	// printf is reliably present on macOS + Linux and does not buffer output.
	cmd := exec.Command("printf", "hello-from-pty\\n")
	ptmx, err := pty.StartWithSize(cmd, &pty.Winsize{Rows: 24, Cols: 80})
	if err != nil {
		t.Fatalf("pty.StartWithSize: %v", err)
	}
	t.Cleanup(func() {
		_ = ptmx.Close()
		_ = cmd.Wait()
	})

	// Read until EOF or 1s elapsed, feeding bytes into the emulator like
	// readPTYCmd does in production.
	done := make(chan struct{})
	go func() {
		defer close(done)
		buf := make([]byte, 4096)
		for {
			n, err := ptmx.Read(buf)
			if n > 0 {
				_, _ = em.Write(buf[:n])
			}
			if err == io.EOF || err != nil {
				return
			}
		}
	}()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("timed out reading from PTY")
	}

	out := ansi.Strip(em.Render())
	if !strings.Contains(out, "hello-from-pty") {
		t.Fatalf("rendered output missing PTY content; got:\n%s", out)
	}
}

// TestVTReplyToPrimaryDeviceAttributes confirms the emulator writes a DA1
// response into its input pipe when it receives \x1b[c, which any well-
// behaved terminal client (tmux included) waits on before drawing. If we
// don't forward those bytes back to the child's PTY, the child hangs and
// the pane stays blank — that's exactly what `tmux attach-session` does.
//
// Critical: em.Write itself BLOCKS on the DA1 handler trying to push the
// reply into its (unread) input pipe — meaning if we don't have a reader
// running before we Write, our PTY read loop deadlocks. That's the
// production bug.
func TestVTReplyToPrimaryDeviceAttributes(t *testing.T) {
	em := vt.NewEmulator(80, 24)
	defer em.Close()

	// Reader must be running BEFORE we Write, otherwise the synchronous
	// reply write inside em.Write blocks on a full io.Pipe.
	got := make(chan []byte, 1)
	go func() {
		buf := make([]byte, 64)
		n, _ := em.Read(buf)
		got <- buf[:n]
	}()

	if _, err := em.Write([]byte("\x1b[c")); err != nil {
		t.Fatalf("write DA1 query: %v", err)
	}

	select {
	case reply := <-got:
		if len(reply) == 0 {
			t.Fatal("emulator wrote no DA1 reply — client would hang")
		}
		if !strings.HasPrefix(string(reply), "\x1b[") {
			t.Fatalf("DA1 reply not a CSI sequence: %q", reply)
		}
		t.Logf("DA1 reply: %q", reply)
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for DA1 reply")
	}
}

// TestVTWriteDeadlocksWithoutPipeReader is the negative version: it proves
// that em.Write hangs if nothing is draining em.Read. This is the failure
// mode our pumpEmulatorReplies goroutine exists to prevent.
func TestVTWriteDeadlocksWithoutPipeReader(t *testing.T) {
	em := vt.NewEmulator(80, 24)
	defer em.Close()

	done := make(chan struct{})
	go func() {
		// Fill the pipe with multiple DA1 queries — a single write may fit
		// but repeated unanswered queries definitely won't.
		for range 50 {
			_, _ = em.Write([]byte("\x1b[c"))
		}
		close(done)
	}()

	select {
	case <-done:
		t.Fatal("em.Write completed without a reader — pipe buffering is larger than expected; the production deadlock theory may still hold for tmux which queries repeatedly")
	case <-time.After(200 * time.Millisecond):
		// Expected: we deadlocked, which is the bug we're fixing.
	}
}

// TestVTRenderHasExpectedLineCount makes sure em.Render() returns one line per
// screen row even when most of the screen is empty.
func TestVTRenderHasExpectedLineCount(t *testing.T) {
	em := vt.NewEmulator(40, 10)
	_, _ = em.Write([]byte("top"))

	rendered := em.Render()
	lineCount := strings.Count(rendered, "\n") + 1
	if lineCount != 10 {
		t.Fatalf("expected 10 lines in Render output, got %d (output=%q)", lineCount, rendered)
	}
}
