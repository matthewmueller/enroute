package ast_test

import (
	"testing"

	"github.com/matryer/is"
	"github.com/matthewmueller/enroute/internal/parser"
)

func TestSplit(t *testing.T) {
	is := is.New(t)
	route, err := parser.Parse("/{name}")
	is.NoErr(err)
	is.Equal(route.Sections.At(0), "/")
	is.Equal(route.Sections.At(1), "{slot}")
	subsections := route.Sections.Split(1)
	is.Equal(len(subsections), 2)
	is.Equal(subsections[0].At(0), "/")
	is.Equal(subsections[0].At(1), "")
	is.Equal(subsections[1].At(0), "{slot}")
	is.Equal(subsections[1].At(1), "")
}

func TestSplitCopy(t *testing.T) {
	is := is.New(t)
	route, err := parser.Parse("/slower")
	is.NoErr(err)
	is.Equal(route.String(), "/slower")
	parts := route.Sections.Split(5)
	is.Equal(route.String(), "/slower")
	is.Equal(parts[0].String(), "/slow")
	is.Equal(parts[1].String(), "er")
}

func expandEqual(t *testing.T, route string, expects ...string) {
	t.Helper()
	t.Run(route, func(t *testing.T) {
		t.Helper()
		if len(expects) == 0 {
			t.Fatal("expected at least one expect")
		}
		r, err := parser.Parse(route)
		if err != nil {
			if len(expects) == 1 && err.Error() == expects[0] {
				return
			}
			t.Fatal(err)
		}
		routes := r.Expand()
		if len(routes) != len(expects) {
			t.Fatalf("expected %d routes, got %d", len(expects), len(routes))
		}
		for i, route := range routes {
			if route.String() != expects[i] {
				t.Fatalf("expected %q, got %q", expects[i], route.String())
			}
		}
	})
}

func TestExpand(t *testing.T) {
	expandEqual(t, "/{name}", "/{name}")
	expandEqual(t, "/{name?}", "/", "/{name}")
	expandEqual(t, "/first/{name?}", "/first", "/first/{name}")
	expandEqual(t, "/{first?}/{last?}", "optional slots must be at the end of the path")
	expandEqual(t, "/{name*}", "/", "/{name*}")
	expandEqual(t, "/first/{name*}", "/first", "/first/{name*}")
	expandEqual(t, "/{first*}/{last*}", "wildcard slots must be at the end of the path")
	expandEqual(t, "/first/{last*}", "/first", "/first/{last*}")
}

func equalLCP(t testing.TB, route1, route2 string, expect int) {
	is := is.New(t)
	is.Helper()
	r1, err := parser.Parse(route1)
	is.NoErr(err)
	r2, err := parser.Parse(route2)
	is.NoErr(err)
	is.Equal(r1.Sections.LongestCommonPrefix(r2.Sections), expect)
}

func TestLCP(t *testing.T) {
	equalLCP(t, "", "", 0)
	equalLCP(t, "/x", "/x", 2)
	equalLCP(t, "/x", "/y", 1)
	equalLCP(t, "/x", "/x/y", 2)
	equalLCP(t, "/x/y", "/x/y", 4)
	equalLCP(t, "/x/y", "/x/z", 3)

	equalLCP(t, "/{a}", "/x", 1)
	equalLCP(t, "/{a}", "/{b}", 2)
	equalLCP(t, "/x{number}", "/x-{custom}", 2)
}
