package binarycookies

import (
	"bytes"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var update = flag.Bool("update", false, "rewrite the golden files in testdata/golden")

// goldenFile is the JSON layout of testdata/golden. Other implementations
// compare themselves against these files, so changing the layout means
// bumping SchemaVersion.
type goldenFile struct {
	SchemaVersion int          `json:"schema_version"`
	Pages         []goldenPage `json:"pages"`
}

type goldenPage struct {
	Cookies []goldenCookie `json:"cookies"`
}

type goldenCookie struct {
	Domain      string `json:"domain"`
	Name        string `json:"name"`
	Path        string `json:"path"`
	Value       string `json:"value"`
	Comment     string `json:"comment"`
	Flags       uint32 `json:"flags"`
	Secure      bool   `json:"secure"`
	HTTPOnly    bool   `json:"http_only"`
	ExpiresUnix int64  `json:"expires_unix"`
	CreatedUnix int64  `json:"created_unix"`
}

// TestGolden pins the decoded form of every Safari sample in testdata/vectors.
func TestGolden(t *testing.T) {
	paths, err := filepath.Glob(filepath.Join("testdata", "vectors", "*.binarycookies"))
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) == 0 {
		t.Fatal("no test vectors found")
	}
	for _, path := range paths {
		name := strings.TrimSuffix(filepath.Base(path), ".binarycookies")
		goldenPath := filepath.Join("testdata", "golden", name+".golden.json")
		t.Run(name, func(t *testing.T) {
			got := renderGolden(t, mustDecode(t, readFile(t, path)))
			if *update {
				if err := os.WriteFile(goldenPath, got, 0o644); err != nil {
					t.Fatal(err)
				}
				return
			}
			if want := readFile(t, goldenPath); !bytes.Equal(got, want) {
				t.Errorf("%s no longer matches %s (run go test -update if the change is intended):\n%s",
					path, goldenPath, got)
			}
		})
	}
}

func renderGolden(t *testing.T, f *File) []byte {
	t.Helper()
	g := goldenFile{SchemaVersion: 2, Pages: make([]goldenPage, 0, len(f.Pages))}
	for _, p := range f.Pages {
		gp := goldenPage{Cookies: make([]goldenCookie, 0, len(p.Cookies))}
		for _, c := range p.Cookies {
			gp.Cookies = append(gp.Cookies, goldenCookie{
				Domain:      c.Domain,
				Name:        c.Name,
				Path:        c.Path,
				Value:       c.Value,
				Comment:     c.Comment,
				Flags:       uint32(c.Flags),
				Secure:      c.Secure(),
				HTTPOnly:    c.HTTPOnly(),
				ExpiresUnix: c.Expires.Unix(),
				CreatedUnix: c.Created.Unix(),
			})
		}
		g.Pages = append(g.Pages, gp)
	}
	blob, err := json.MarshalIndent(g, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	return append(blob, '\n')
}
