package commands

import "testing"

func TestCleanReport(t *testing.T) {
	tests := map[string]string{
		"  login   is\tbroken\n": "login is broken",
		"already clean":          "already clean",
		"   ":                    "",
		"line one\n\nline  two":  "line one line two",
	}

	for in, want := range tests {
		if got := cleanReport(in); got != want {
			t.Errorf("cleanReport(%q) = %q, want %q", in, got, want)
		}
	}
}
