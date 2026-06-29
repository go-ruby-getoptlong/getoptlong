// Copyright (c) the go-ruby-getoptlong/getoptlong authors
//
// SPDX-License-Identifier: BSD-3-Clause

// Package getoptlong is a pure-Go (no cgo) reimplementation of Ruby's
// GetoptLong — the getopt-style command-line option parser shipped with MRI
// (ruby/getoptlong 0.2.1, the version bundled with Ruby 4.0.5).
//
// It is a faithful, byte-for-byte port of MRI's option-scanning algorithm: the
// same long/short/abbreviation matching, the same =-joined and separate
// arguments, the same bundled short flags, the same `--` terminator, the same
// three ordering modes (REQUIRE_ORDER, PERMUTE, RETURN_IN_ORDER) and the same
// error taxonomy (InvalidOption, MissingArgument, NeedlessArgument,
// AmbiguousOption) with MRI-identical messages.
//
// Unlike Ruby's GetoptLong, which mutates the global ARGV, this package operates
// on an explicit argument slice owned by the Parser, so it is reusable and free
// of global state. The host (for example go-embedded-ruby's `rbgo`) binds ARGV
// to a Parser and reads the remaining, non-option arguments back from
// Parser.Args after processing.
//
// The scanning is pure compute and needs no Ruby runtime; the package has no
// dependencies.
package getoptlong

import (
	"fmt"
	"io"
	"strings"
)

// ArgumentFlag describes whether an option takes an argument. The values match
// Ruby's GetoptLong::NO_ARGUMENT / REQUIRED_ARGUMENT / OPTIONAL_ARGUMENT.
type ArgumentFlag int

const (
	// NoArgument is GetoptLong::NO_ARGUMENT — the option takes no argument.
	NoArgument ArgumentFlag = 0
	// RequiredArgument is GetoptLong::REQUIRED_ARGUMENT — the option must be
	// followed by an argument.
	RequiredArgument ArgumentFlag = 1
	// OptionalArgument is GetoptLong::OPTIONAL_ARGUMENT — the option may be
	// followed by an argument.
	OptionalArgument ArgumentFlag = 2
)

// Ordering selects how options and non-option arguments are interpreted. The
// values match Ruby's GetoptLong::REQUIRE_ORDER / PERMUTE / RETURN_IN_ORDER.
type Ordering int

const (
	// RequireOrder stops option processing at the first non-option word; every
	// word after it is a non-option word (GetoptLong::REQUIRE_ORDER).
	RequireOrder Ordering = 0
	// Permute lets options and non-option arguments mix in any order; the
	// non-option words are collected and returned at the end (GetoptLong::PERMUTE).
	// This is the default for a new Parser.
	Permute Ordering = 1
	// ReturnInOrder treats every word as an option: a non-option word is
	// returned as an option whose name is "" and whose value is the word
	// (GetoptLong::RETURN_IN_ORDER).
	ReturnInOrder Ordering = 2
)

// Internal status, mirroring GetoptLong's STATUS_YET / STATUS_STARTED /
// STATUS_TERMINATED.
type status int

const (
	statusYet status = iota
	statusStarted
	statusTerminated
)

// Error is the base of the GetoptLong error taxonomy; it corresponds to
// GetoptLong::Error (a StandardError in Ruby). Every parsing error returned by
// GetNext wraps one of the four concrete kinds and reports an MRI-identical
// message via Error.Error.
type Error struct {
	// Kind is the concrete error class (InvalidOption, MissingArgument,
	// NeedlessArgument, or AmbiguousOption).
	Kind ErrorKind
	// Message is the POSIX-format message, identical to Ruby's
	// GetoptLong#error_message.
	Message string
}

func (e *Error) Error() string { return e.Message }

// ErrorKind enumerates the concrete GetoptLong error classes.
type ErrorKind int

const (
	// InvalidOption is GetoptLong::InvalidOption — an unrecognised option.
	InvalidOption ErrorKind = iota
	// MissingArgument is GetoptLong::MissingArgument — a required argument is
	// absent.
	MissingArgument
	// NeedlessArgument is GetoptLong::NeedlessArgument — an argument was given
	// to a NO_ARGUMENT option.
	NeedlessArgument
	// AmbiguousOption is GetoptLong::AmbiguousOption — an abbreviation matches
	// more than one option.
	AmbiguousOption
)

// String returns the Ruby class name of the error kind, e.g.
// "GetoptLong::InvalidOption".
func (k ErrorKind) String() string {
	switch k {
	case InvalidOption:
		return "GetoptLong::InvalidOption"
	case MissingArgument:
		return "GetoptLong::MissingArgument"
	case NeedlessArgument:
		return "GetoptLong::NeedlessArgument"
	case AmbiguousOption:
		return "GetoptLong::AmbiguousOption"
	default:
		return "GetoptLong::Error"
	}
}

// SpecError reports an invalid option specification passed to New or SetOptions;
// it corresponds to the ArgumentError that Ruby's GetoptLong.new / #set_options
// raise. Its message is MRI-identical.
type SpecError struct {
	Message string
}

func (e *SpecError) Error() string { return e.Message }

// Option defines a single option: a canonical name, its aliases, and the
// argument flag. It is the Go form of one array passed to Ruby's
// GetoptLong.new, e.g. ["--name", "-n", GetoptLong::REQUIRED_ARGUMENT].
//
// Names is the list of name and aliases; the first element is the canonical
// name reported by GetNext. Each name must be either "-" followed by a single
// non-hyphen character, or "--" followed by one or more characters.
type Option struct {
	// Names is the canonical name (first) followed by any aliases.
	Names []string
	// Flag is the argument flag for this option.
	Flag ArgumentFlag
}

// Parser scans an argument slice for options. It is the Go counterpart of a
// GetoptLong instance. Configure it with New (or NewParser + SetOptions), set
// the ordering and quiet mode if needed, then call GetNext repeatedly. After
// processing, Args holds the remaining non-option arguments (the equivalent of
// the ARGV that Ruby leaves behind).
type Parser struct {
	// Args is the working argument slice. GetNext consumes options from the
	// front; on termination the collected non-option arguments are restored
	// here, so after a full scan Args is exactly the remaining ARGV.
	Args []string

	// ProgName is prepended to error messages written to ErrorWriter, matching
	// Ruby's $0. If empty, no program-name prefix is written.
	ProgName string

	// ErrorWriter receives error messages unless Quiet is set. Ruby writes them
	// to $stderr; if ErrorWriter is nil, messages are discarded (but GetNext
	// still returns the error). Set it to os.Stderr to mirror Ruby exactly.
	ErrorWriter io.Writer

	ordering       Ordering
	canonicalNames map[string]string       // name/alias -> canonical name
	argumentFlags  map[string]ArgumentFlag // name/alias -> flag
	nameOrder      []string                // registration order of every name/alias
	quiet          bool
	status         status
	err            *Error
	restSingles    string   // rest of catenated short options
	nonOptionArgs  []string // collected non-option words (PERMUTE)
}

// NewParser returns an empty Parser in the default PERMUTE ordering with no
// options configured. Add options with SetOptions. The argument slice defaults
// to nil; assign Parser.Args before scanning.
func NewParser() *Parser {
	return &Parser{
		ordering:       Permute,
		canonicalNames: map[string]string{},
		argumentFlags:  map[string]ArgumentFlag{},
		status:         statusYet,
	}
}

// New returns a Parser configured to scan args for the given options, in the
// default PERMUTE ordering. It is the counterpart of GetoptLong.new. A bad
// option specification returns a *SpecError.
func New(args []string, options ...Option) (*Parser, error) {
	p := NewParser()
	p.Args = args
	if err := p.SetOptions(options...); err != nil {
		return nil, err
	}
	return p, nil
}

// Ordering returns the current ordering setting.
func (p *Parser) Ordering() Ordering { return p.ordering }

// SetOrdering sets the ordering. It corresponds to GetoptLong#ordering=. It
// fails once option processing has started. Unlike Ruby, this package does not
// consult the POSIXLY_CORRECT environment variable; pass RequireOrder
// explicitly for POSIX behaviour.
func (p *Parser) SetOrdering(o Ordering) error {
	if p.status != statusYet {
		return &SpecError{Message: "invoke ordering=, but option processing has already started"}
	}
	if o != RequireOrder && o != Permute && o != ReturnInOrder {
		return &SpecError{Message: fmt.Sprintf("invalid ordering `%d'", int(o))}
	}
	p.ordering = o
	return nil
}

// Quiet reports whether error messages are suppressed.
func (p *Parser) Quiet() bool { return p.quiet }

// SetQuiet sets quiet mode; when true, error messages are not written to
// ErrorWriter (GetNext still returns the error). It corresponds to
// GetoptLong#quiet=.
func (p *Parser) SetQuiet(q bool) { p.quiet = q }

// Err returns the error that terminated processing, or nil if none. It
// corresponds to GetoptLong#error.
func (p *Parser) Err() *Error { return p.err }

// ErrorMessage returns the message of the error that terminated processing, or
// "" if none. It corresponds to GetoptLong#error_message.
func (p *Parser) ErrorMessage() string {
	if p.err == nil {
		return ""
	}
	return p.err.Message
}

// Terminated reports whether option processing has finished. It corresponds to
// GetoptLong#terminated?.
func (p *Parser) Terminated() bool { return p.status == statusTerminated }

// SetOptions replaces the configured options. It corresponds to
// GetoptLong#set_options. It fails once processing has started, or if any
// specification is invalid (returning a *SpecError with an MRI-identical
// message). On any spec error the option tables are left empty, as in Ruby.
func (p *Parser) SetOptions(options ...Option) error {
	if p.status != statusYet {
		return &SpecError{Message: "invoke set_options, but option processing has already started"}
	}
	clear(p.canonicalNames)
	clear(p.argumentFlags)
	p.nameOrder = nil

	for _, opt := range options {
		canonical := ""
		for _, name := range opt.Names {
			if !validOptionName(name) {
				clear(p.canonicalNames)
				clear(p.argumentFlags)
				p.nameOrder = nil
				return &SpecError{Message: fmt.Sprintf("an invalid option `%s'", name)}
			}
			if _, ok := p.canonicalNames[name]; ok {
				clear(p.canonicalNames)
				clear(p.argumentFlags)
				p.nameOrder = nil
				return &SpecError{Message: fmt.Sprintf("option redefined `%s'", name)}
			}
			if canonical == "" {
				canonical = name
			}
			p.canonicalNames[name] = canonical
			p.argumentFlags[name] = opt.Flag
			p.nameOrder = append(p.nameOrder, name)
		}
		if canonical == "" {
			return &SpecError{Message: "no option name"}
		}
	}
	return nil
}

// validOptionName reports whether name matches Ruby's /\A-([^-]|-.+)\z/: a
// single hyphen plus one non-hyphen character, or two hyphens plus one or more
// characters.
func validOptionName(name string) bool {
	if !strings.HasPrefix(name, "-") {
		return false
	}
	rest := name[1:]
	if rest == "" {
		return false
	}
	if rest[0] != '-' {
		// -X : exactly one non-hyphen character.
		return len([]rune(rest)) == 1
	}
	// --... : at least one character after the second hyphen.
	return len(rest) >= 2
}

// terminate ends option processing, restoring the collected non-option
// arguments to the front of Args (Ruby unshifts them onto ARGV).
func (p *Parser) terminate() {
	if p.status == statusTerminated {
		return
	}
	p.status = statusTerminated
	if len(p.nonOptionArgs) > 0 {
		p.Args = append(append([]string{}, p.nonOptionArgs...), p.Args...)
		p.nonOptionArgs = nil
	}
}

// setError records and reports an error, then returns it. It mirrors
// GetoptLong#set_error: the message is written to ErrorWriter (prefixed with
// ProgName) unless Quiet, and processing is poisoned so later GetNext calls
// return nothing.
func (p *Parser) setError(kind ErrorKind, message string) *Error {
	if !p.quiet && p.ErrorWriter != nil {
		if p.ProgName != "" {
			fmt.Fprintf(p.ErrorWriter, "%s: %s\n", p.ProgName, message)
		} else {
			fmt.Fprintf(p.ErrorWriter, "%s\n", message)
		}
	}
	e := &Error{Kind: kind, Message: message}
	p.err = e
	return e
}

// GetNext returns the next option as (name, argument, ok). name is the canonical
// option name (never an alias); argument is its value ("" when the option takes
// no argument or an optional argument was absent). ok is false when there are no
// more options — either the input is exhausted or an error occurred; check
// err for the latter. GetNext corresponds to GetoptLong#get / #get_option.
//
// On error, GetNext returns ("", "", false) together with a non-nil *Error and
// records it (see Err / ErrorMessage).
func (p *Parser) GetNext() (name, argument string, ok bool, err error) {
	if p.err != nil {
		return "", "", false, nil
	}
	switch p.status {
	case statusYet:
		p.status = statusStarted
	case statusTerminated:
		return "", "", false, nil
	}

	var arg string

	// Get next option argument.
	switch {
	case len(p.restSingles) > 0:
		arg = "-" + p.restSingles
	case len(p.Args) == 0:
		p.terminate()
		return "", "", false, nil
	case p.ordering == Permute:
		for len(p.Args) > 0 && !matchHyphenDot(p.Args[0]) {
			p.nonOptionArgs = append(p.nonOptionArgs, p.Args[0])
			p.Args = p.Args[1:]
		}
		if len(p.Args) == 0 {
			p.terminate()
			return "", "", false, nil
		}
		arg, p.Args = p.Args[0], p.Args[1:]
	case p.ordering == RequireOrder:
		if !matchHyphenDot(p.Args[0]) {
			p.terminate()
			return "", "", false, nil
		}
		arg, p.Args = p.Args[0], p.Args[1:]
	default: // ReturnInOrder
		arg, p.Args = p.Args[0], p.Args[1:]
	}

	// The `--' terminator.
	if arg == "--" && len(p.restSingles) == 0 {
		p.terminate()
		return "", "", false, nil
	}

	var optionName, optionArgument string

	if pattern, isLong := longPattern(arg); isLong && len(p.restSingles) == 0 {
		// Long option: starts with `--', up to (not including) any `='.
		optionName = pattern
		if _, ok := p.canonicalNames[optionName]; !ok {
			// Not registered verbatim — try abbreviation matching. Iterate in
			// registration order so the ambiguity list matches MRI's
			// @canonical_names.each_key order byte-for-byte.
			var matches []string
			for _, key := range p.nameOrder {
				if strings.HasPrefix(key, pattern) {
					optionName = key
					matches = append(matches, key)
				}
			}
			switch {
			case len(matches) >= 2:
				e := p.setError(AmbiguousOption,
					fmt.Sprintf("option `%s' is ambiguous between %s", arg, strings.Join(matches, ", ")))
				return "", "", false, e
			case len(matches) == 0:
				e := p.setError(InvalidOption, fmt.Sprintf("unrecognized option `%s'", arg))
				return "", "", false, e
			}
		}

		// Check an argument to the option.
		switch p.argumentFlags[optionName] {
		case RequiredArgument:
			if v, ok := eqValue(arg); ok {
				optionArgument = v
			} else if len(p.Args) > 0 {
				optionArgument, p.Args = p.Args[0], p.Args[1:]
			} else {
				e := p.setError(MissingArgument, fmt.Sprintf("option `%s' requires an argument", arg))
				return "", "", false, e
			}
		case OptionalArgument:
			if v, ok := eqValue(arg); ok {
				optionArgument = v
			} else if len(p.Args) > 0 && !matchHyphenDot(p.Args[0]) {
				optionArgument, p.Args = p.Args[0], p.Args[1:]
			} else {
				optionArgument = ""
			}
		default: // NoArgument
			if _, ok := eqValue(arg); ok {
				e := p.setError(NeedlessArgument, fmt.Sprintf("option `%s' doesn't allow an argument", optionName))
				return "", "", false, e
			}
		}
	} else if name, ch, rest, isShort := shortPattern(arg); isShort {
		// Short option: `-x'; short options may be catenated (`-lg' == `-l -g').
		optionName, p.restSingles = name, rest

		if _, ok := p.canonicalNames[optionName]; ok {
			switch p.argumentFlags[optionName] {
			case RequiredArgument:
				if len(p.restSingles) > 0 {
					optionArgument, p.restSingles = p.restSingles, ""
				} else if len(p.Args) > 0 {
					optionArgument, p.Args = p.Args[0], p.Args[1:]
				} else {
					e := p.setError(MissingArgument, fmt.Sprintf("option requires an argument -- %s", ch))
					return "", "", false, e
				}
			case OptionalArgument:
				if len(p.restSingles) > 0 {
					optionArgument, p.restSingles = p.restSingles, ""
				} else if len(p.Args) > 0 && !matchHyphenDot(p.Args[0]) {
					optionArgument, p.Args = p.Args[0], p.Args[1:]
				} else {
					optionArgument = ""
				}
			}
		} else {
			e := p.setError(InvalidOption, fmt.Sprintf("invalid option -- %s", ch))
			return "", "", false, e
		}
	} else {
		// Non-option argument. Only RETURN_IN_ORDER reaches here.
		return "", arg, true, nil
	}

	return p.canonicalNames[optionName], optionArgument, true, nil
}

// Each calls fn with each successive option until processing ends. It
// corresponds to GetoptLong#each / #each_option. It stops and returns the error
// if GetNext reports one.
func (p *Parser) Each(fn func(name, argument string)) error {
	for {
		name, argument, ok, err := p.GetNext()
		if err != nil {
			return err
		}
		if !ok {
			return nil
		}
		fn(name, argument)
	}
}

// matchHyphenDot reports whether s matches Ruby's /\A-./ : a hyphen followed by
// at least one more character.
func matchHyphenDot(s string) bool {
	return len(s) >= 2 && s[0] == '-'
}

// longPattern returns the long-option name for arg (the `--...` prefix up to,
// but not including, any `=`), and whether arg is a long option. It mirrors
// Ruby's /\A(--[^=]+)/.
func longPattern(arg string) (string, bool) {
	if !strings.HasPrefix(arg, "--") {
		return "", false
	}
	body := arg[2:]
	if body == "" || body[0] == '=' {
		// "--" is handled earlier; "--=..." has an empty name -> not a long match.
		return "", false
	}
	if i := strings.IndexByte(body, '='); i >= 0 {
		return "--" + body[:i], true
	}
	return "--" + body, true
}

// shortPattern decomposes arg as a short option, mirroring Ruby's
// /\A(-(.))(.*)/m: name is `-X`, ch is the single option character, rest is the
// remaining catenated singles. It returns ok=false when arg is not of that form.
func shortPattern(arg string) (name, ch, rest string, ok bool) {
	if len(arg) < 2 || arg[0] != '-' {
		return "", "", "", false
	}
	r := []rune(arg[1:])
	ch = string(r[0])
	name = "-" + ch
	rest = string(r[1:])
	return name, ch, rest, true
}

// eqValue returns the part of a long option after the first `=`, and whether an
// `=` was present, mirroring Ruby's /=(.*)/m.
func eqValue(arg string) (string, bool) {
	if i := strings.IndexByte(arg, '='); i >= 0 {
		return arg[i+1:], true
	}
	return "", false
}
