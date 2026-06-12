// MIT License
//
// Copyright (c) 2026 Bitwise Media Group
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

package cli

import "strings"

// ExtractFlags splits a raw argv (from a DisableFlagParsing cobra command)
// into the flags dotty owns and the args to forward verbatim to the proxied
// program.
//
// spec maps a long flag name (no dashes) to whether it takes a value. Owned
// flags are recognized in both --name=value and --name value forms anywhere
// before a bare "--"; everything else — including the "--" and all that
// follows it — lands in rest in its original order. A lone -h or --help before
// "--" sets help instead of being forwarded, which is how proxy commands keep
// DESIGN's "--help is always handled by dotty" promise.
func ExtractFlags(args []string, spec map[string]bool) (own map[string]string, rest []string, help bool) {
	own = map[string]string{}
	rest = []string{}
	for i := 0; i < len(args); i++ {
		arg := args[i]

		if arg == "--" {
			rest = append(rest, args[i:]...)
			break
		}
		if arg == "-h" || arg == "--help" {
			help = true
			continue
		}

		name, value, hasValue := cutFlag(arg)
		if name == "" {
			rest = append(rest, arg)
			continue
		}
		takesValue, owned := spec[name]
		if !owned {
			rest = append(rest, arg)
			continue
		}
		switch {
		case hasValue: // --name=value
			own[name] = value
		case takesValue && i+1 < len(args): // --name value
			own[name] = args[i+1]
			i++
		case takesValue: // --name at end of argv; record as set-but-empty
			own[name] = ""
		default: // boolean owned flag
			own[name] = "true"
		}
	}
	return own, rest, help
}

// cutFlag parses a "--name" or "--name=value" token. It returns name == ""
// for anything that is not a long flag (short flags and positionals belong to
// the proxied program).
func cutFlag(arg string) (name, value string, hasValue bool) {
	body, ok := strings.CutPrefix(arg, "--")
	if !ok || body == "" {
		return "", "", false
	}
	name, value, hasValue = strings.Cut(body, "=")
	return name, value, hasValue
}
