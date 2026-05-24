package cli

import (
	"flag"
	"fmt"
	"strings"
)

// parseError wraps an error from splitArgs/flag.Parse with the raw stderr
// the FlagSet would have printed. Used by callers to surface a useful hint
// in the "usage:" line instead of a generic blob.
type parseError struct {
	err    error
	stderr string
}

func (p *parseError) Error() string {
	if p.stderr != "" {
		return strings.TrimSpace(p.stderr)
	}
	return p.err.Error()
}
func (p *parseError) Unwrap() error { return p.err }

// flagSet wraps flag.FlagSet with two extras: a record of which flags were
// actually passed (so `set` commands can tell "omitted" from "empty"), and
// a pre-parser that lets positionals appear anywhere in the argv (stdlib
// flag stops at the first positional).
type flagSet struct {
	fs       *flag.FlagSet
	seen     map[string]bool
	posArgs  []string
	knownVal map[string]bool // names of flags that take a value
	sink     *stderrSink
}

func newFlagSet(name string) *flagSet {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	sink := &stderrSink{}
	fs.SetOutput(sink)
	return &flagSet{fs: fs, knownVal: map[string]bool{}, sink: sink}
}

func (f *flagSet) string(name, usage string) *string {
	f.knownVal[name] = true
	return f.fs.String(name, "", usage)
}

// parse splits args into flag tokens and positional tokens, then runs
// flag.Parse on the flag tokens. Positionals can appear anywhere in argv.
func (f *flagSet) parse(args []string) error {
	flagToks, pos, err := splitArgs(args, f.knownVal)
	if err != nil {
		return &parseError{err: err}
	}
	f.posArgs = pos
	if err := f.fs.Parse(flagToks); err != nil {
		return &parseError{err: err, stderr: f.sink.buf.String()}
	}
	f.seen = map[string]bool{}
	f.fs.Visit(func(fl *flag.Flag) { f.seen[fl.Name] = true })
	return nil
}

func (f *flagSet) provided(name string) bool { return f.seen[name] }
func (f *flagSet) positional() []string      { return f.posArgs }

// splitArgs walks args once and partitions them into flag tokens (to feed
// flag.FlagSet) vs positional tokens. Recognised forms:
//
//	--name=value   -> single flag token
//	--name value   -> two flag tokens (only when "name" is in known)
//	--             -> end of flags; rest is positional
//	anything else  -> positional
func splitArgs(args []string, known map[string]bool) (flagToks, pos []string, err error) {
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "--" {
			pos = append(pos, args[i+1:]...)
			return
		}
		if !strings.HasPrefix(a, "-") || a == "-" {
			pos = append(pos, a)
			continue
		}
		// strip leading dashes
		body := strings.TrimLeft(a, "-")
		name, _, hasEq := strings.Cut(body, "=")
		if !known[name] {
			return nil, nil, fmt.Errorf("unknown flag --%s", name)
		}
		if hasEq {
			flagToks = append(flagToks, a)
			continue
		}
		// --name value form: consume next arg as the value
		if i+1 >= len(args) {
			return nil, nil, fmt.Errorf("flag --%s requires a value", name)
		}
		flagToks = append(flagToks, a, args[i+1])
		i++
	}
	return
}

// stderrSink captures flag's default error output so callers can surface
// the actual error text ("flag provided but not defined: -rooot") instead
// of a generic usage blob.
type stderrSink struct{ buf strings.Builder }

func (s *stderrSink) Write(p []byte) (int, error) { return s.buf.Write(p) }

func trim(s string) string { return strings.TrimSpace(s) }
