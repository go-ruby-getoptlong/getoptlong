// Copyright (c) the go-ruby-getoptlong/getoptlong authors
//
// SPDX-License-Identifier: BSD-3-Clause

package getoptlong

import (
	"encoding/json"
	"os/exec"
	"strings"
	"testing"
)

// rubyBin locates a usable `ruby` whose GetoptLong matches the version this
// package targets (>= 4.0; the bundled getoptlong 0.2.1 grammar). The oracle
// tests skip themselves when ruby is absent (the qemu cross-arch lanes and the
// Windows lane) or older, so the deterministic suite alone drives the 100% gate
// there.
func rubyBin(t *testing.T) string {
	t.Helper()
	path, err := exec.LookPath("ruby")
	if err != nil {
		t.Skip("ruby not on PATH; skipping MRI oracle")
	}
	out, err := exec.Command(path, "-e", "print RUBY_VERSION").Output()
	if err != nil {
		t.Skipf("cannot determine ruby version: %v", err)
	}
	if v := string(out); v < "4.0" {
		t.Skipf("ruby %s < 4.0; skipping MRI oracle", v)
	}
	return path
}

// oracleScript drives MRI's GetoptLong over argv with the given option specs and
// ordering, printing each (name, argument) pair plus the final ARGV and any
// error as a single JSON object. It $stdout.binmode's so Windows text-mode never
// rewrites the bytes (the cross-platform lesson), and reads its inputs from
// stdin (also binmode) so no shell quoting of argv is involved.
const oracleScript = `
$stdout.binmode
$stdin.binmode
require 'getoptlong'
require 'json'
input = JSON.parse($stdin.read)
ARGV.replace(input['argv'])
specs = input['specs'].map do |s|
  flag = {0 => GetoptLong::NO_ARGUMENT, 1 => GetoptLong::REQUIRED_ARGUMENT,
          2 => GetoptLong::OPTIONAL_ARGUMENT}[s['flag']]
  s['names'] + [flag]
end
$0 = 'prog'
opts = GetoptLong.new(*specs)
opts.ordering = {0 => GetoptLong::REQUIRE_ORDER, 1 => GetoptLong::PERMUTE,
                 2 => GetoptLong::RETURN_IN_ORDER}[input['ordering']]
opts.quiet = true
results = []
err = nil
begin
  opts.each { |name, arg| results << [name, arg] }
rescue GetoptLong::Error => e
  err = {'class' => e.class.name, 'message' => e.message}
end
print JSON.generate({'results' => results, 'argv' => ARGV, 'error' => err})
`

// oracleScenario is one differential case: an argv, a set of option specs, and
// an ordering, run through both this package and MRI.
type oracleScenario struct {
	name     string
	argv     []string
	specs    []Option
	ordering Ordering
}

// rubyOutput is the JSON shape oracleScript prints.
type rubyOutput struct {
	Results [][2]string `json:"results"`
	Argv    []string    `json:"argv"`
	Error   *struct {
		Class   string `json:"class"`
		Message string `json:"message"`
	} `json:"error"`
}

// runMRI executes oracleScript against bin for the scenario and returns MRI's
// decoded output.
func runMRI(t *testing.T, bin string, s oracleScenario) rubyOutput {
	t.Helper()
	type specJSON struct {
		Names []string `json:"names"`
		Flag  int      `json:"flag"`
	}
	argv := s.argv
	if argv == nil {
		argv = []string{} // marshal as [] so Ruby's ARGV.replace gets an array
	}
	in := struct {
		Argv     []string   `json:"argv"`
		Specs    []specJSON `json:"specs"`
		Ordering int        `json:"ordering"`
	}{Argv: argv, Ordering: int(s.ordering)}
	for _, o := range s.specs {
		in.Specs = append(in.Specs, specJSON{Names: o.Names, Flag: int(o.Flag)})
	}
	payload, err := json.Marshal(in)
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(bin, "-e", oracleScript)
	cmd.Stdin = strings.NewReader(string(payload))
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("ruby error: %v\noutput: %s", err, out)
	}
	var ro rubyOutput
	if err := json.Unmarshal(out, &ro); err != nil {
		t.Fatalf("decode MRI output %q: %v", out, err)
	}
	return ro
}

// rubyErrClass maps a Go ErrorKind to the Ruby class name MRI reports.
func rubyErrClass(k ErrorKind) string { return k.String() }

// oracleScenarios spans every behaviour: all three orderings, long/short,
// abbreviation, =-joined / separate / bundled arguments, the `--` terminator,
// and each error class.
func oracleScenarios() []oracleScenario {
	xyz := []Option{reqOpt("--xxx", "-x"), optOpt("--yyy", "-y"), noOpt("--zzz", "-z")}
	abbr := []Option{noOpt("--xxx"), noOpt("--xyz")}
	return []oracleScenario{
		{"permute", []string{"--xxx", "Foo", "--yyy", "Bar", "Baz", "--zzz", "Bat", "Bam"}, xyz, Permute},
		{"require_order", []string{"--xxx", "Foo", "Bar", "--xxx", "Baz", "--yyy", "Bat", "-zzz"}, xyz, RequireOrder},
		{"return_in_order", []string{"Foo", "--xxx", "Bar", "Baz", "--zzz", "Bat", "Bam"}, xyz, ReturnInOrder},
		{"abbrev", []string{"--xxx", "--xx", "--xyz", "--xy"}, abbr, Permute},
		{"long_eq", []string{"--xxx=foo"}, xyz, Permute},
		{"long_eq_empty", []string{"--xxx="}, xyz, Permute},
		{"short_attached", []string{"-xfoo"}, xyz, Permute},
		{"short_separate", []string{"-x", "foo"}, xyz, Permute},
		{"bundled", []string{"-zz", "-z"}, xyz, Permute},
		{"optional_short_last", []string{"-y"}, xyz, Permute},
		{"optional_short_value", []string{"-y", "v"}, xyz, Permute},
		{"optional_short_then_opt", []string{"-y", "-z"}, xyz, Permute},
		{"required_takes_optionlike", []string{"--xxx", "--yyy"}, xyz, Permute},
		{"terminator", []string{"--zzz", "--", "--xxx", "foo"}, xyz, Permute},
		{"repeat", []string{"--zzz", "--zzz", "-z"}, xyz, Permute},
		{"err_ambiguous", []string{"--x"}, abbr, Permute},
		{"err_invalid_long", []string{"--zzz"}, abbr, Permute},
		{"err_missing_long", []string{"--xxx"}, []Option{reqOpt("--xxx")}, Permute},
		{"err_needless_long", []string{"--zzz=foo"}, xyz, Permute},
		{"err_invalid_short", []string{"-q"}, xyz, Permute},
		{"err_missing_short", []string{"-x"}, []Option{reqOpt("-x")}, Permute},
		{"empty", nil, xyz, Permute},
		{"all_nonoption_permute", []string{"a", "b", "c"}, xyz, Permute},
		{"all_nonoption_require", []string{"a", "b", "c"}, xyz, RequireOrder},
	}
}

// TestOracleDifferential runs every scenario through both this package and the
// system ruby and asserts the option stream, the leftover argv, and the error
// (class + message) match byte-for-byte.
func TestOracleDifferential(t *testing.T) {
	bin := rubyBin(t)
	for _, s := range oracleScenarios() {
		t.Run(s.name, func(t *testing.T) {
			// This package.
			p, err := New(append([]string(nil), s.argv...), s.specs...)
			if err != nil {
				t.Fatalf("New: %v", err)
			}
			if err := p.SetOrdering(s.ordering); err != nil {
				t.Fatalf("SetOrdering: %v", err)
			}
			p.SetQuiet(true)
			var goResults [][2]string
			var goErr *Error
			for {
				name, arg, ok, e := p.GetNext()
				if e != nil {
					goErr = e.(*Error)
					break
				}
				if !ok {
					break
				}
				goResults = append(goResults, [2]string{name, arg})
			}
			goArgv := p.Args

			// MRI.
			ro := runMRI(t, bin, s)

			if !eqPairs(goResults, ro.Results) {
				t.Errorf("results:\n  go  = %v\n  mri = %v", goResults, ro.Results)
			}
			if !eqStrings(goArgv, ro.Argv) {
				t.Errorf("leftover argv:\n  go  = %v\n  mri = %v", goArgv, ro.Argv)
			}
			switch {
			case goErr == nil && ro.Error != nil:
				t.Errorf("go had no error, MRI raised %s: %s", ro.Error.Class, ro.Error.Message)
			case goErr != nil && ro.Error == nil:
				t.Errorf("go raised %s: %s, MRI had none", rubyErrClass(goErr.Kind), goErr.Message)
			case goErr != nil && ro.Error != nil:
				if rubyErrClass(goErr.Kind) != ro.Error.Class {
					t.Errorf("error class:\n  go  = %s\n  mri = %s", rubyErrClass(goErr.Kind), ro.Error.Class)
				}
				if goErr.Message != ro.Error.Message {
					t.Errorf("error message:\n  go  = %q\n  mri = %q", goErr.Message, ro.Error.Message)
				}
			}
		})
	}
}

func eqPairs(a [][2]string, b [][2]string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func eqStrings(a, b []string) bool {
	if len(a) != len(b) {
		// Treat nil and empty as equal.
		if len(a) == 0 && len(b) == 0 {
			return true
		}
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// TestOracleSpecErrors checks that the construction-time ArgumentError messages
// match MRI's for each invalid specification.
func TestOracleSpecErrors(t *testing.T) {
	bin := rubyBin(t)
	const script = `
$stdout.binmode
$stdin.binmode
require 'getoptlong'
require 'json'
specs = JSON.parse($stdin.read).map do |s|
  s.map { |e| e.is_a?(Integer) ? {0=>GetoptLong::NO_ARGUMENT,1=>GetoptLong::REQUIRED_ARGUMENT,2=>GetoptLong::OPTIONAL_ARGUMENT}[e] : e }
end
begin
  GetoptLong.new(*specs)
  print JSON.generate({'error' => nil})
rescue => e
  print JSON.generate({'error' => {'class' => e.class.name, 'message' => e.message}})
end
`
	cases := []struct {
		name  string
		specs []Option
		// spec is the raw JSON the script consumes; built from specs.
	}{
		{"invalid_name", []Option{{Names: []string{"--xxx", "badname"}, Flag: NoArgument}}},
		{"redefined", []Option{noOpt("--xxx"), noOpt("--xxx")}},
		{"bare_hyphen", []Option{noOpt("-")}},
		{"short_too_long", []Option{noOpt("-xy")}},
		{"non_hyphen", []Option{noOpt("xxx")}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// Build the raw spec JSON: each Option -> [name..., flagInt].
			var raw [][]any
			for _, o := range c.specs {
				row := make([]any, 0, len(o.Names)+1)
				for _, n := range o.Names {
					row = append(row, n)
				}
				row = append(row, int(o.Flag))
				raw = append(raw, row)
			}
			payload, _ := json.Marshal(raw)
			cmd := exec.Command(bin, "-e", script)
			cmd.Stdin = strings.NewReader(string(payload))
			out, err := cmd.Output()
			if err != nil {
				t.Fatalf("ruby: %v\n%s", err, out)
			}
			var ro struct {
				Error *struct {
					Class, Message string
				} `json:"error"`
			}
			if err := json.Unmarshal(out, &ro); err != nil {
				t.Fatalf("decode %q: %v", out, err)
			}
			if ro.Error == nil {
				t.Fatalf("MRI accepted spec %v; expected ArgumentError", c.specs)
			}

			_, gerr := New(nil, c.specs...)
			se, ok := gerr.(*SpecError)
			if !ok {
				t.Fatalf("go error = %v (%T), want *SpecError", gerr, gerr)
			}
			if ro.Error.Class != "ArgumentError" {
				t.Errorf("MRI class = %s, want ArgumentError", ro.Error.Class)
			}
			if se.Message != ro.Error.Message {
				t.Errorf("message:\n  go  = %q\n  mri = %q", se.Message, ro.Error.Message)
			}
		})
	}
}
