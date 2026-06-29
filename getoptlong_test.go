// Copyright (c) the go-ruby-getoptlong/getoptlong authors
//
// SPDX-License-Identifier: BSD-3-Clause

package getoptlong

import (
	"bytes"
	"reflect"
	"testing"
)

// result is one (name, argument) pair yielded by GetNext, used in the golden
// tables below.
type result struct {
	name string
	arg  string
}

// run drives a Parser to exhaustion, collecting every (name, argument) pair and
// the terminating error (if any), and returns them with the leftover Args.
func run(p *Parser) (got []result, leftover []string, err error) {
	for {
		name, argument, ok, e := p.GetNext()
		if e != nil {
			err = e
			break
		}
		if !ok {
			break
		}
		got = append(got, result{name, argument})
	}
	return got, p.Args, err
}

// reqOpt / optOpt / noOpt are option-spec constructors used throughout.
func reqOpt(names ...string) Option { return Option{Names: names, Flag: RequiredArgument} }
func optOpt(names ...string) Option { return Option{Names: names, Flag: OptionalArgument} }
func noOpt(names ...string) Option  { return Option{Names: names, Flag: NoArgument} }

func TestPermute(t *testing.T) {
	p, err := New([]string{"--xxx", "Foo", "--yyy", "Bar", "Baz", "--zzz", "Bat", "Bam"},
		reqOpt("--xxx", "-x"), optOpt("--yyy", "-y"), noOpt("--zzz", "-z"))
	if err != nil {
		t.Fatal(err)
	}
	got, leftover, e := run(p)
	if e != nil {
		t.Fatalf("unexpected error: %v", e)
	}
	want := []result{{"--xxx", "Foo"}, {"--yyy", "Bar"}, {"--zzz", ""}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("options = %v, want %v", got, want)
	}
	if wantLeft := []string{"Baz", "Bat", "Bam"}; !reflect.DeepEqual(leftover, wantLeft) {
		t.Errorf("leftover = %v, want %v", leftover, wantLeft)
	}
	if p.Ordering() != Permute {
		t.Errorf("ordering = %v, want Permute", p.Ordering())
	}
}

func TestRequireOrder(t *testing.T) {
	p, _ := New([]string{"--xxx", "Foo", "Bar", "--xxx", "Baz", "--yyy", "Bat", "-zzz"},
		reqOpt("--xxx", "-x"), optOpt("--yyy", "-y"), noOpt("--zzz", "-z"))
	if err := p.SetOrdering(RequireOrder); err != nil {
		t.Fatal(err)
	}
	got, leftover, e := run(p)
	if e != nil {
		t.Fatalf("unexpected error: %v", e)
	}
	want := []result{{"--xxx", "Foo"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("options = %v, want %v", got, want)
	}
	wantLeft := []string{"Bar", "--xxx", "Baz", "--yyy", "Bat", "-zzz"}
	if !reflect.DeepEqual(leftover, wantLeft) {
		t.Errorf("leftover = %v, want %v", leftover, wantLeft)
	}
}

func TestReturnInOrder(t *testing.T) {
	p, _ := New([]string{"Foo", "--xxx", "Bar", "Baz", "--zzz", "Bat", "Bam"},
		reqOpt("--xxx", "-x"), noOpt("--zzz", "-z"))
	if err := p.SetOrdering(ReturnInOrder); err != nil {
		t.Fatal(err)
	}
	got, leftover, e := run(p)
	if e != nil {
		t.Fatalf("unexpected error: %v", e)
	}
	want := []result{
		{"", "Foo"}, {"--xxx", "Bar"}, {"", "Baz"}, {"--zzz", ""}, {"", "Bat"}, {"", "Bam"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("options = %v, want %v", got, want)
	}
	if len(leftover) != 0 {
		t.Errorf("leftover = %v, want empty", leftover)
	}
}

func TestAbbreviation(t *testing.T) {
	p, _ := New([]string{"--xxx", "--xx", "--xyz", "--xy"}, noOpt("--xxx"), noOpt("--xyz"))
	got, _, e := run(p)
	if e != nil {
		t.Fatalf("unexpected error: %v", e)
	}
	want := []result{{"--xxx", ""}, {"--xxx", ""}, {"--xyz", ""}, {"--xyz", ""}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("options = %v, want %v", got, want)
	}
}

func TestLongEqualsJoined(t *testing.T) {
	p, _ := New([]string{"--xxx=foo"}, reqOpt("--xxx"))
	got, _, _ := run(p)
	if want := []result{{"--xxx", "foo"}}; !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestLongEqualsEmptyValue(t *testing.T) {
	p, _ := New([]string{"--xxx="}, reqOpt("--xxx"))
	got, _, _ := run(p)
	if want := []result{{"--xxx", ""}}; !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestOptionalLongFollowedByOption(t *testing.T) {
	p, _ := New([]string{"--yyy", "--zzz"}, optOpt("--yyy"), noOpt("--zzz"))
	got, _, _ := run(p)
	want := []result{{"--yyy", ""}, {"--zzz", ""}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestOptionalLongFollowedByValue(t *testing.T) {
	p, _ := New([]string{"--yyy", "foo"}, optOpt("--yyy"))
	got, _, _ := run(p)
	if want := []result{{"--yyy", "foo"}}; !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestOptionalLongEqualsOverridesNext(t *testing.T) {
	p, _ := New([]string{"--yyy=v", "foo"}, optOpt("--yyy"))
	got, leftover, _ := run(p)
	if want := []result{{"--yyy", "v"}}; !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
	if want := []string{"foo"}; !reflect.DeepEqual(leftover, want) {
		t.Errorf("leftover = %v, want %v", leftover, want)
	}
}

func TestRequiredLongTakesNextEvenIfOptionLike(t *testing.T) {
	p, _ := New([]string{"--xxx", "--yyy"}, reqOpt("--xxx"))
	got, _, _ := run(p)
	if want := []result{{"--xxx", "--yyy"}}; !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestBundledShort(t *testing.T) {
	p, _ := New([]string{"-xyz"}, noOpt("-x"), noOpt("-y"), noOpt("-z"))
	got, _, _ := run(p)
	want := []result{{"-x", ""}, {"-y", ""}, {"-z", ""}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestShortAttachedArg(t *testing.T) {
	p, _ := New([]string{"-xfoo"}, reqOpt("-x"))
	got, _, _ := run(p)
	if want := []result{{"-x", "foo"}}; !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestShortSeparateArg(t *testing.T) {
	p, _ := New([]string{"-x", "foo"}, reqOpt("-x"))
	got, _, _ := run(p)
	if want := []result{{"-x", "foo"}}; !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestShortOptionalAttached(t *testing.T) {
	p, _ := New([]string{"-yfoo"}, optOpt("-y"))
	got, _, _ := run(p)
	if want := []result{{"-y", "foo"}}; !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestShortOptionalSeparate(t *testing.T) {
	p, _ := New([]string{"-y", "foo"}, optOpt("-y"))
	got, leftover, _ := run(p)
	if want := []result{{"-y", "foo"}}; !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
	if len(leftover) != 0 {
		t.Errorf("leftover = %v, want empty", leftover)
	}
}

func TestShortOptionalLast(t *testing.T) {
	p, _ := New([]string{"-y"}, optOpt("-y"))
	got, _, _ := run(p)
	if want := []result{{"-y", ""}}; !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestShortOptionalFollowedByOption(t *testing.T) {
	p, _ := New([]string{"-y", "-z"}, optOpt("-y"), noOpt("-z"))
	got, _, _ := run(p)
	want := []result{{"-y", ""}, {"-z", ""}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestBundledShortWithRequiredArg(t *testing.T) {
	// -xy where -x takes a required argument: y becomes its argument.
	p, _ := New([]string{"-xy"}, reqOpt("-x"), noOpt("-y"))
	got, _, _ := run(p)
	if want := []result{{"-x", "y"}}; !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestTerminator(t *testing.T) {
	p, _ := New([]string{"--xxx", "--", "--yyy", "foo"}, noOpt("--xxx"), noOpt("--yyy"))
	got, leftover, _ := run(p)
	if want := []result{{"--xxx", ""}}; !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
	if want := []string{"--yyy", "foo"}; !reflect.DeepEqual(leftover, want) {
		t.Errorf("leftover = %v, want %v", leftover, want)
	}
}

func TestAliasReportsCanonical(t *testing.T) {
	p, _ := New([]string{"-x", "--yy"}, noOpt("--xxx", "-x"), noOpt("--yyy", "--yy"))
	got, _, _ := run(p)
	want := []result{{"--xxx", ""}, {"--yyy", ""}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestEmptyArgs(t *testing.T) {
	p, _ := New(nil, noOpt("--xxx"))
	got, leftover, _ := run(p)
	if len(got) != 0 {
		t.Errorf("got %v, want empty", got)
	}
	if len(leftover) != 0 {
		t.Errorf("leftover = %v, want empty", leftover)
	}
	if !p.Terminated() {
		t.Error("expected terminated after exhausting empty args")
	}
}

func TestEach(t *testing.T) {
	p, _ := New([]string{"--xxx", "-y", "v"}, noOpt("--xxx"), reqOpt("-y"))
	var got []result
	if err := p.Each(func(name, argument string) {
		got = append(got, result{name, argument})
	}); err != nil {
		t.Fatal(err)
	}
	want := []result{{"--xxx", ""}, {"-y", "v"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestUnicodeShort(t *testing.T) {
	// A multi-byte short-option name and a multi-byte catenated rest.
	p, _ := New([]string{"-é", "-éz"}, noOpt("-é"), noOpt("-z"))
	got, _, e := run(p)
	if e != nil {
		t.Fatalf("unexpected error: %v", e)
	}
	want := []result{{"-é", ""}, {"-é", ""}, {"-z", ""}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

// --- Error cases ---

func errCase(t *testing.T, args []string, opts []Option, wantKind ErrorKind, wantMsg string) {
	t.Helper()
	p, err := New(args, opts...)
	if err != nil {
		t.Fatalf("spec error: %v", err)
	}
	var buf bytes.Buffer
	p.ProgName = "prog"
	p.ErrorWriter = &buf
	_, _, e := run(p)
	ge, ok := e.(*Error)
	if !ok {
		t.Fatalf("error = %v (%T), want *Error", e, e)
	}
	if ge.Kind != wantKind {
		t.Errorf("kind = %v, want %v", ge.Kind, wantKind)
	}
	if ge.Message != wantMsg {
		t.Errorf("message = %q, want %q", ge.Message, wantMsg)
	}
	if got := buf.String(); got != "prog: "+wantMsg+"\n" {
		t.Errorf("stderr = %q, want %q", got, "prog: "+wantMsg+"\n")
	}
	if p.Err() == nil || p.ErrorMessage() != wantMsg {
		t.Errorf("Err()/ErrorMessage() not recorded: %v / %q", p.Err(), p.ErrorMessage())
	}
	// After an error, further GetNext yields nothing.
	if name, arg, ok, e2 := p.GetNext(); ok || e2 != nil || name != "" || arg != "" {
		t.Errorf("post-error GetNext = (%q,%q,%v,%v), want empty", name, arg, ok, e2)
	}
}

func TestErrAmbiguous(t *testing.T) {
	errCase(t, []string{"--x"}, []Option{noOpt("--xxx"), noOpt("--xyz")},
		AmbiguousOption, "option `--x' is ambiguous between --xxx, --xyz")
}

func TestErrInvalidLong(t *testing.T) {
	errCase(t, []string{"--zzz"}, []Option{noOpt("--xxx")},
		InvalidOption, "unrecognized option `--zzz'")
}

func TestErrMissingLong(t *testing.T) {
	errCase(t, []string{"--xxx"}, []Option{reqOpt("--xxx")},
		MissingArgument, "option `--xxx' requires an argument")
}

func TestErrNeedlessLong(t *testing.T) {
	errCase(t, []string{"--xxx=foo"}, []Option{noOpt("--xxx")},
		NeedlessArgument, "option `--xxx' doesn't allow an argument")
}

func TestErrInvalidShort(t *testing.T) {
	errCase(t, []string{"-q"}, []Option{noOpt("-x")},
		InvalidOption, "invalid option -- q")
}

func TestErrMissingShort(t *testing.T) {
	errCase(t, []string{"-x"}, []Option{reqOpt("-x")},
		MissingArgument, "option requires an argument -- x")
}

func TestErrAmbiguousThreeWay(t *testing.T) {
	// Registration order is preserved in the ambiguity list.
	errCase(t, []string{"--xy"}, []Option{noOpt("--xyz"), noOpt("--xya"), noOpt("--xyb")},
		AmbiguousOption, "option `--xy' is ambiguous between --xyz, --xya, --xyb")
}

func TestQuietSuppressesOutput(t *testing.T) {
	p, _ := New([]string{"--zzz"}, noOpt("--xxx"))
	var buf bytes.Buffer
	p.ProgName = "prog"
	p.ErrorWriter = &buf
	p.SetQuiet(true)
	if !p.Quiet() {
		t.Error("Quiet() = false after SetQuiet(true)")
	}
	_, _, e := run(p)
	if e == nil {
		t.Fatal("expected error")
	}
	if buf.Len() != 0 {
		t.Errorf("stderr = %q, want empty in quiet mode", buf.String())
	}
}

func TestErrorWriterNilDiscards(t *testing.T) {
	// No ErrorWriter and not quiet: message is discarded but error still returned.
	p, _ := New([]string{"--zzz"}, noOpt("--xxx"))
	_, _, e := run(p)
	if e == nil {
		t.Fatal("expected error")
	}
}

func TestErrorNoProgName(t *testing.T) {
	p, _ := New([]string{"--zzz"}, noOpt("--xxx"))
	var buf bytes.Buffer
	p.ErrorWriter = &buf // ProgName empty
	_, _, _ = run(p)
	if want := "unrecognized option `--zzz'\n"; buf.String() != want {
		t.Errorf("stderr = %q, want %q", buf.String(), want)
	}
}

func TestEachReturnsError(t *testing.T) {
	p, _ := New([]string{"--zzz"}, noOpt("--xxx"))
	err := p.Each(func(string, string) { t.Error("block must not run") })
	if err == nil {
		t.Fatal("Each should return the error")
	}
}

// --- Spec (construction) errors ---

func specErrCase(t *testing.T, opts []Option, wantMsg string) {
	t.Helper()
	_, err := New(nil, opts...)
	se, ok := err.(*SpecError)
	if !ok {
		t.Fatalf("error = %v (%T), want *SpecError", err, err)
	}
	if se.Message != wantMsg {
		t.Errorf("message = %q, want %q", se.Message, wantMsg)
	}
}

func TestSpecInvalidName(t *testing.T) {
	specErrCase(t, []Option{{Names: []string{"--xxx", "badname"}, Flag: NoArgument}},
		"an invalid option `badname'")
}

func TestSpecRedefined(t *testing.T) {
	specErrCase(t, []Option{noOpt("--xxx"), noOpt("--xxx")}, "option redefined `--xxx'")
}

func TestSpecNoOptionName(t *testing.T) {
	specErrCase(t, []Option{{Names: nil, Flag: NoArgument}}, "no option name")
}

func TestSpecEmptyName(t *testing.T) {
	specErrCase(t, []Option{noOpt("")}, "an invalid option `'")
}

func TestSpecBareHyphen(t *testing.T) {
	specErrCase(t, []Option{noOpt("-")}, "an invalid option `-'")
}

func TestSpecDoubleHyphenOnly(t *testing.T) {
	specErrCase(t, []Option{noOpt("--")}, "an invalid option `--'")
}

func TestSpecShortTooLong(t *testing.T) {
	// "-xy" is not a valid single-hyphen name (must be exactly one char).
	specErrCase(t, []Option{noOpt("-xy")}, "an invalid option `-xy'")
}

func TestSpecNonHyphen(t *testing.T) {
	specErrCase(t, []Option{noOpt("xxx")}, "an invalid option `xxx'")
}

func TestSpecValidUnicodeShort(t *testing.T) {
	// A single multi-byte rune is a valid short name.
	if _, err := New(nil, noOpt("-é")); err != nil {
		t.Errorf("unexpected spec error for -é: %v", err)
	}
}

// --- State-machine guards ---

func TestSetOrderingAfterStart(t *testing.T) {
	p, _ := New([]string{"--xxx"}, noOpt("--xxx"))
	p.GetNext() // starts processing
	if err := p.SetOrdering(RequireOrder); err == nil {
		t.Fatal("SetOrdering after start should fail")
	} else if se, ok := err.(*SpecError); !ok ||
		se.Message != "invoke ordering=, but option processing has already started" {
		t.Errorf("error = %v", err)
	}
}

func TestSetOptionsAfterStart(t *testing.T) {
	p, _ := New([]string{"--xxx"}, noOpt("--xxx"))
	p.GetNext()
	if err := p.SetOptions(noOpt("--yyy")); err == nil {
		t.Fatal("SetOptions after start should fail")
	} else if se, ok := err.(*SpecError); !ok ||
		se.Message != "invoke set_options, but option processing has already started" {
		t.Errorf("error = %v", err)
	}
}

func TestSetOrderingInvalid(t *testing.T) {
	p := NewParser()
	if err := p.SetOrdering(Ordering(99)); err == nil {
		t.Fatal("expected invalid-ordering error")
	} else if se, ok := err.(*SpecError); !ok || se.Message != "invalid ordering `99'" {
		t.Errorf("error = %v", err)
	}
}

func TestSetOrderingValid(t *testing.T) {
	p := NewParser()
	for _, o := range []Ordering{RequireOrder, Permute, ReturnInOrder} {
		if err := p.SetOrdering(o); err != nil {
			t.Fatalf("SetOrdering(%v): %v", o, err)
		}
		if p.Ordering() != o {
			t.Errorf("ordering = %v, want %v", p.Ordering(), o)
		}
	}
}

func TestNewParserAndSetOptions(t *testing.T) {
	p := NewParser()
	p.Args = []string{"--xxx", "v"}
	if err := p.SetOptions(reqOpt("--xxx")); err != nil {
		t.Fatal(err)
	}
	got, _, _ := run(p)
	if want := []result{{"--xxx", "v"}}; !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestSetOptionsReplaces(t *testing.T) {
	p := NewParser()
	if err := p.SetOptions(noOpt("--aaa")); err != nil {
		t.Fatal(err)
	}
	// Replace: --aaa no longer known, --bbb is.
	if err := p.SetOptions(noOpt("--bbb")); err != nil {
		t.Fatal(err)
	}
	p.Args = []string{"--bbb"}
	got, _, _ := run(p)
	if want := []result{{"--bbb", ""}}; !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestSpecErrorClearsTables(t *testing.T) {
	p := NewParser()
	if err := p.SetOptions(noOpt("--good")); err != nil {
		t.Fatal(err)
	}
	// A later bad spec must clear the tables (MRI rescue clause).
	if err := p.SetOptions(noOpt("--ok"), noOpt("badname")); err == nil {
		t.Fatal("expected spec error")
	}
	if len(p.canonicalNames) != 0 || len(p.argumentFlags) != 0 || len(p.nameOrder) != 0 {
		t.Errorf("tables not cleared after spec error: %v / %v / %v",
			p.canonicalNames, p.argumentFlags, p.nameOrder)
	}
}

func TestGetNextNoMoreAfterTerminated(t *testing.T) {
	p, _ := New([]string{"--xxx"}, noOpt("--xxx"))
	run(p) // exhaust
	if !p.Terminated() {
		t.Fatal("expected terminated")
	}
	if name, arg, ok, err := p.GetNext(); ok || err != nil || name != "" || arg != "" {
		t.Errorf("GetNext after terminated = (%q,%q,%v,%v)", name, arg, ok, err)
	}
}

func TestErrorKindString(t *testing.T) {
	cases := map[ErrorKind]string{
		InvalidOption:    "GetoptLong::InvalidOption",
		MissingArgument:  "GetoptLong::MissingArgument",
		NeedlessArgument: "GetoptLong::NeedlessArgument",
		AmbiguousOption:  "GetoptLong::AmbiguousOption",
		ErrorKind(99):    "GetoptLong::Error",
	}
	for k, want := range cases {
		if got := k.String(); got != want {
			t.Errorf("ErrorKind(%d).String() = %q, want %q", int(k), got, want)
		}
	}
}

func TestEmptyOrderingNoArgsReturnInOrder(t *testing.T) {
	// RETURN_IN_ORDER with no args terminates immediately.
	p, _ := New(nil, noOpt("-x"))
	p.SetOrdering(ReturnInOrder)
	got, leftover, _ := run(p)
	if len(got) != 0 || len(leftover) != 0 {
		t.Errorf("got %v leftover %v, want both empty", got, leftover)
	}
}

func TestRequireOrderImmediateNonOption(t *testing.T) {
	// First word is a non-option: REQUIRE_ORDER stops at once.
	p, _ := New([]string{"foo", "--xxx"}, noOpt("--xxx"))
	p.SetOrdering(RequireOrder)
	got, leftover, _ := run(p)
	if len(got) != 0 {
		t.Errorf("got %v, want empty", got)
	}
	if want := []string{"foo", "--xxx"}; !reflect.DeepEqual(leftover, want) {
		t.Errorf("leftover = %v, want %v", leftover, want)
	}
}

func TestLongOnlyDoubleHyphenValueIsNonOption(t *testing.T) {
	// "--=v" has an empty long name; it is not a long option. In PERMUTE it is
	// not "-." either (it starts with "--" so it IS "-."), so it is scanned as a
	// long pattern that fails longPattern -> falls to short -> "-" + "-=v"...
	// Verify it surfaces as an invalid short option, matching MRI.
	p, _ := New([]string{"--=v"}, noOpt("--xxx"))
	var buf bytes.Buffer
	p.ProgName = "prog"
	p.ErrorWriter = &buf
	_, _, e := run(p)
	ge, ok := e.(*Error)
	if !ok {
		t.Fatalf("error = %v (%T)", e, e)
	}
	// MRI: shortPattern("--=v") -> name "--", ch "-", so "invalid option -- -".
	if ge.Kind != InvalidOption || ge.Message != "invalid option -- -" {
		t.Errorf("got (%v, %q), want (InvalidOption, %q)", ge.Kind, ge.Message, "invalid option -- -")
	}
}

func TestErrorErrorMethod(t *testing.T) {
	e := &Error{Kind: InvalidOption, Message: "boom"}
	if e.Error() != "boom" {
		t.Errorf("Error() = %q, want %q", e.Error(), "boom")
	}
}

func TestSpecErrorErrorMethod(t *testing.T) {
	e := &SpecError{Message: "spec boom"}
	if e.Error() != "spec boom" {
		t.Errorf("Error() = %q, want %q", e.Error(), "spec boom")
	}
}

func TestErrorMessageEmptyWhenNoError(t *testing.T) {
	p := NewParser()
	if p.ErrorMessage() != "" {
		t.Errorf("ErrorMessage() = %q, want empty", p.ErrorMessage())
	}
	if p.Err() != nil {
		t.Errorf("Err() = %v, want nil", p.Err())
	}
}

func TestTerminateIdempotent(t *testing.T) {
	// Terminating an already-terminated parser is a no-op (the early return in
	// terminate). Exhaust once, then GetNext again hits the terminated branch
	// without re-running termination logic.
	p, _ := New(nil, noOpt("-x"))
	p.terminate()
	p.terminate() // second call must be a no-op
	if !p.Terminated() {
		t.Fatal("expected terminated")
	}
}

func TestMatchHyphenDotShortNonOption(t *testing.T) {
	// A bare "-" (length 1) in PERMUTE is a non-option word and is collected.
	p, _ := New([]string{"-", "--xxx"}, noOpt("--xxx"))
	got, leftover, _ := run(p)
	if want := []result{{"--xxx", ""}}; !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
	if want := []string{"-"}; !reflect.DeepEqual(leftover, want) {
		t.Errorf("leftover = %v, want %v", leftover, want)
	}
}
