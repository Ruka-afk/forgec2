package payload

import (
	"go/parser"
	"go/token"
	"strings"
	"testing"
)

// TestStripUTLSCredsProducesParsableSource pins the invariant that matters most
// about a line-splicing stripper: the result has to still be valid Go.
//
// The existing checks assert on content (no grpc, no utlsCreds, the shared
// dialers survive), which all pass even if the import block is left dangling.
// That gap is exactly how the broken matcher went unnoticed: stripUTLSCreds
// was failing outright for unrelated reasons, so nobody was reading the output.
//
// It also pins the specific regression: the grpc import is currently the only
// entry in its block, so its line carries the closing bracket. A matcher that
// only accepts the bare form silently stops finding it, and the guard meant to
// "fail loudly on refactor" instead blocked every slim build.
func TestStripUTLSCredsProducesParsableSource(t *testing.T) {
	src, err := payloadFS.ReadFile("agent/transport_utls.go")
	if err != nil {
		t.Fatalf("read template: %v", err)
	}
	out, err := stripUTLSCreds(src)
	if err != nil {
		t.Fatalf("stripUTLSCreds: %v", err)
	}

	fset := token.NewFileSet()
	if _, err := parser.ParseFile(fset, "transport_utls.go", out, parser.AllErrors); err != nil {
		t.Fatalf("stripped source does not parse (slim builds would fail to compile): %v\n---\n%s",
			err, tail(string(out), 400))
	}

	text := string(out)
	if strings.Contains(text, "grpc") {
		t.Fatalf("stripped source still mentions grpc:\n%s", tail(text, 400))
	}
	// A dangling import block is the classic line-splice failure.
	if strings.Contains(text, "import (") && strings.Contains(text, "import ()") {
		t.Fatal("stripped source has an empty import block")
	}
	if strings.Count(text, "\nimport (") != strings.Count(text, "\n)") {
		t.Fatal("stripped source has unbalanced import parentheses")
	}
	// fmt was only needed by the grpc tail; dropping it is required for the
	// slim copy to compile, so assert it actually went.
	if strings.Contains(text, "\t\"fmt\"\n") || strings.Contains(text, "\t\"fmt\"]\n") {
		t.Fatal("stripped source kept the fmt import but the grpc tail that used it is gone")
	}
}

// TestStripImportLineAcceptsBothBlockForms is the unit-level counterpart: the
// matcher must handle an import alone in its block and one among siblings.
func TestStripImportLineAcceptsBothBlockForms(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "alone in its block, bracket on the same line",
			in:   "import (\n\t\"context\"\n\t\"google.golang.org/grpc/credentials\"]\n)\n",
			want: "import (\n\t\"context\"\n)\n",
		},
		{
			name: "among siblings, bracket on the next line",
			in:   "import (\n\t\"context\"\n\t\"google.golang.org/grpc/credentials\"\n\t\"net\"\n)\n",
			want: "import (\n\t\"context\"\n\t\"net\"\n)\n",
		},
		{
			name: "absent",
			in:   "import (\n\t\"context\"\n)\n",
			want: "import (\n\t\"context\"\n)\n",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, found := stripImportLine(tc.in, "google.golang.org/grpc/credentials")
			if tc.name == "absent" {
				if found {
					t.Fatal("reported a strip that did not happen")
				}
			} else if !found {
				t.Fatalf("did not find the import in:\n%s", tc.in)
			}
			if got != tc.want {
				t.Fatalf("got:\n%q\nwant:\n%q", got, tc.want)
			}
		})
	}
}

func tail(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return "..." + s[len(s)-n:]
}
