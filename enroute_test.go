package enroute_test

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"slices"

	"github.com/matryer/is"
	"github.com/matthewmueller/diff"
	"github.com/matthewmueller/enroute"
)

func insertEqual(t *testing.T, tree *enroute.Tree, route string, expected string) {
	t.Helper()
	t.Run(route, func(t *testing.T) {
		t.Helper()
		if err := tree.Insert(route, "something"); err != nil {
			if err.Error() == expected {
				return
			}
			t.Fatal(err)
		}
		actual := strings.TrimSpace(tree.String())
		expected = strings.ReplaceAll(strings.TrimSpace(expected), "\t", "")
		diff.TestString(t, actual, expected)
	})
}

// https://en.wikipedia.org/wiki/Radix_tree#Insertion
func TestWikipediaInsert(t *testing.T) {
	tree := enroute.New()
	insertEqual(t, tree, "/test", `
		/test [from=/test]
	`)
	insertEqual(t, tree, "/slow", `
		/
		•test [from=/test]
		•slow [from=/slow]
	`)
	insertEqual(t, tree, "/water", `
		/
		•test [from=/test]
		•slow [from=/slow]
		•water [from=/water]
	`)
	insertEqual(t, tree, "/slower", `
		/
		•test [from=/test]
		•slow [from=/slow]
		•••••er [from=/slower]
		•water [from=/water]
	`)
	tree = enroute.New()
	insertEqual(t, tree, "/tester", `
		/tester [from=/tester]
	`)
	insertEqual(t, tree, "/test", `
		/test [from=/test]
		•••••er [from=/tester]
	`)
	tree = enroute.New()
	insertEqual(t, tree, "/test", `
		/test [from=/test]
	`)
	insertEqual(t, tree, "/team", `
		/te
		•••st [from=/test]
		•••am [from=/team]
	`)
	insertEqual(t, tree, "/toast", `
		/t
		••e
		•••st [from=/test]
		•••am [from=/team]
		••oast [from=/toast]
	`)
}

func TestSampleInsert(t *testing.T) {
	tree := enroute.New()
	insertEqual(t, tree, "/hello/{name}", `
		/hello/{name} [from=/hello/{name}]
	`)
	insertEqual(t, tree, "/howdy/{name}/", `
		/h
		••ello/{name} [from=/hello/{name}]
		••owdy/{name} [from=/howdy/{name}]
	`)
	insertEqual(t, tree, "/hello/{name}/elsewhere", `
		/h
		••ello/{name} [from=/hello/{name}]
		•••••••••••••/elsewhere [from=/hello/{name}/elsewhere]
		••owdy/{name} [from=/howdy/{name}]
	`)
	insertEqual(t, tree, "/hello/{name}/admin/", `
		/h
		••ello/{name} [from=/hello/{name}]
		•••••••••••••/
		••••••••••••••elsewhere [from=/hello/{name}/elsewhere]
		••••••••••••••admin [from=/hello/{name}/admin]
		••owdy/{name} [from=/howdy/{name}]
	`)
	insertEqual(t, tree, "/hello/{name}/else/", `
		/h
		••ello/{name} [from=/hello/{name}]
		•••••••••••••/
		••••••••••••••else [from=/hello/{name}/else]
		••••••••••••••••••where [from=/hello/{name}/elsewhere]
		••••••••••••••admin [from=/hello/{name}/admin]
		••owdy/{name} [from=/howdy/{name}]
	`)
}

func TestEqual(t *testing.T) {
	tree := enroute.New()
	insertEqual(t, tree, "/hello/{name}", `
		/hello/{name} [from=/hello/{name}]
	`)
	insertEqual(t, tree, "/hello/{name}", `route already exists "/hello/{name}"`)
	insertEqual(t, tree, "/hello", `
		/hello [from=/hello]
		••••••/{name} [from=/hello/{name}]
	`)
	insertEqual(t, tree, "/hello", `route already exists "/hello"`)
	tree = enroute.New()
	insertEqual(t, tree, "/{name}", `
		/{name} [from=/{name}]
	`)
	insertEqual(t, tree, "/{title}", `route "/{title}" is ambiguous with "/{name}"`)
}

func TestDifferentSlots(t *testing.T) {
	tree := enroute.New()
	insertEqual(t, tree, "/{name}", `
		/{name} [from=/{name}]
	`)
	insertEqual(t, tree, "/{first}/{last}", `
		/{name} [from=/{name}]
		•••••••/{last} [from=/{first}/{last}]
	`)
	insertEqual(t, tree, "/{first}/else", `
		/{name} [from=/{name}]
		•••••••/
		••••••••else [from=/{first}/else]
		••••••••{last} [from=/{first}/{last}]
	`)
	tree = enroute.New()
	insertEqual(t, tree, "/{name}", `
		/{name} [from=/{name}]
	`)
	insertEqual(t, tree, "/else", `
		/
		•else [from=/else]
		•{name} [from=/{name}]
	`)
}

func TestPathAfter(t *testing.T) {
	tree := enroute.New()
	insertEqual(t, tree, "/{name}", `
		/{name} [from=/{name}]
	`)
	insertEqual(t, tree, "/", `
		/ [from=/]
		•{name} [from=/{name}]
	`)
	insertEqual(t, tree, "/first/{name}", `
		/ [from=/]
		•first/{name} [from=/first/{name}]
		•{name} [from=/{name}]
	`)
	insertEqual(t, tree, "/first", `
		/ [from=/]
		•first [from=/first]
		••••••/{name} [from=/first/{name}]
		•{name} [from=/{name}]
	`)
}

func TestOptionals(t *testing.T) {
	tree := enroute.New()
	insertEqual(t, tree, "/{name?}", `
		/ [from=/{name?}]
		•{name} [from=/{name?}]
	`)
	insertEqual(t, tree, "/first/{last?}", `
		/ [from=/{name?}]
		•first [from=/first/{last?}]
		••••••/{last} [from=/first/{last?}]
		•{name} [from=/{name?}]
	`)
	insertEqual(t, tree, "/{first}/{last}", `
		/ [from=/{name?}]
		•first [from=/first/{last?}]
		••••••/{last} [from=/first/{last?}]
		•{name} [from=/{name?}]
		•••••••/{last} [from=/{first}/{last}]
	`)
	insertEqual(t, tree, "/first/else", `
		/ [from=/{name?}]
		•first [from=/first/{last?}]
		••••••/
		•••••••else [from=/first/else]
		•••••••{last} [from=/first/{last?}]
		•{name} [from=/{name?}]
		•••••••/{last} [from=/{first}/{last}]
	`)
}

func TestWildcards(t *testing.T) {
	tree := enroute.New()
	insertEqual(t, tree, "/{name*}", `
		/ [from=/{name*}]
		•{name*} [from=/{name*}]
	`)
	insertEqual(t, tree, "/first/{last*}", `
		/ [from=/{name*}]
		•first [from=/first/{last*}]
		••••••/{last*} [from=/first/{last*}]
		•{name*} [from=/{name*}]
	`)
	insertEqual(t, tree, "/{first}/{last}", `
		/ [from=/{name*}]
		•first [from=/first/{last*}]
		••••••/{last*} [from=/first/{last*}]
		•{name*} [from=/{name*}]
		••••••••/{last} [from=/{first}/{last}]
	`)
	insertEqual(t, tree, "/first/else", `
		/ [from=/{name*}]
		•first [from=/first/{last*}]
		••••••/
		•••••••else [from=/first/else]
		•••••••{last*} [from=/first/{last*}]
		•{name*} [from=/{name*}]
		••••••••/{last} [from=/{first}/{last}]
	`)
}

func TestInsertRegexp(t *testing.T) {
	tree := enroute.New()
	insertEqual(t, tree, "/{name|[A-Z]}", `
		/{name|^[A-Z]$} [from=/{name|^[A-Z]$}]
	`)
	insertEqual(t, tree, "/{name|[A-Z]*}", `regexp "[A-Z]*" must match at least one character`)
	insertEqual(t, tree, "/{path|[0-9]}", `
		/
		•{name|^[A-Z]$} [from=/{name|^[A-Z]$}]
		•{path|^[0-9]$} [from=/{path|^[0-9]$}]
	`)
	insertEqual(t, tree, "/{digits|^[0-9]$}", `route "/{digits|^[0-9]$}" is ambiguous with "/{path|^[0-9]$}"`)
	insertEqual(t, tree, "/first/last", `
		/
		•first/last [from=/first/last]
		•{name|^[A-Z]$} [from=/{name|^[A-Z]$}]
		•{path|^[0-9]$} [from=/{path|^[0-9]$}]
	`)
	insertEqual(t, tree, "/{name}", `
		/
		•first/last [from=/first/last]
		•{name|^[A-Z]$} [from=/{name|^[A-Z]$}]
		•{path|^[0-9]$} [from=/{path|^[0-9]$}]
		•{name} [from=/{name}]
	`)
	insertEqual(t, tree, "/{last*}", `route "/{last*}" is ambiguous with "/{name}"`)
	// TODO: / shouldn't be routable, it got modified by the error above
	insertEqual(t, tree, "/first/{last*}", `
		/ [from=/{last*}]
		•first [from=/first/{last*}]
		••••••/
		•••••••last [from=/first/last]
		•••••••{last*} [from=/first/{last*}]
		•{name|^[A-Z]$} [from=/{name|^[A-Z]$}]
		•{path|^[0-9]$} [from=/{path|^[0-9]$}]
		•{name} [from=/{name}]
	`)
	insertEqual(t, tree, "/{path|[0-9]+}", `
		/ [from=/{last*}]
		•first [from=/first/{last*}]
		••••••/
		•••••••last [from=/first/last]
		•••••••{last*} [from=/first/{last*}]
		•{name|^[A-Z]$} [from=/{name|^[A-Z]$}]
		•{path|^[0-9]$} [from=/{path|^[0-9]$}]
		•{path|^[0-9]+$} [from=/{path|^[0-9]+$}]
		•{name} [from=/{name}]
	`)
}

func TestInsertRegexpSlotFirst(t *testing.T) {
	tree := enroute.New()
	insertEqual(t, tree, "/{name}", `
		/{name} [from=/{name}]
	`)
	insertEqual(t, tree, "/{path|[A-Z]+}", `
		/
		•{path|^[A-Z]+$} [from=/{path|^[A-Z]+$}]
		•{name} [from=/{name}]
	`)
}

func TestRootSwap(t *testing.T) {
	tree := enroute.New()
	insertEqual(t, tree, "/hello", `
		/hello [from=/hello]
	`)
	insertEqual(t, tree, "/", `
		/ [from=/]
		•hello [from=/hello]
	`)
}

func TestPriority(t *testing.T) {
	tree := enroute.New()
	insertEqual(t, tree, "/v{version}", `
	/v{version} [from=/v{version}]
	`)
	insertEqual(t, tree, "/v2", `
	/v
	••2 [from=/v2]
	••{version} [from=/v{version}]
	`)
	tree = enroute.New()
	insertEqual(t, tree, "/v{version}", `
		/v{version} [from=/v{version}]
	`)
	insertEqual(t, tree, "/v{major}.{minor}.{patch}", `
		/v{version} [from=/v{version}]
		•••••••••••.{minor}.{patch} [from=/v{major}.{minor}.{patch}]
	`)
}

func TestSlotSplit(t *testing.T) {
	tree := enroute.New()
	insertEqual(t, tree, "/users/{id}/edit", `
		/users/{id}/edit [from=/users/{id}/edit]
	`)
	insertEqual(t, tree, "/users/settings", `
		/users/
		•••••••settings [from=/users/settings]
		•••••••{id}/edit [from=/users/{id}/edit]
	`)
}

func TestInvalidSlot(t *testing.T) {
	tree := enroute.New()
	insertEqual(t, tree, "/{a}", `
		/{a} [from=/{a}]
	`)
	insertEqual(t, tree, "/{a}{b}", `slot "a" can't have another slot after`)
}

type Routes []Route

type Route struct {
	Route    string
	Requests Requests
}

type Requests []Request

type Request struct {
	Path   string
	Expect string
}

func matchPath(t *testing.T, tree *enroute.Tree, path string, expect string) {
	t.Helper()
	match, err := tree.Match(path)
	if err != nil {
		if err.Error() == expect {
			return
		}
		t.Fatal(err.Error())
	}
	if match.Route != "" && match.Value == "" {
		t.Fatalf("routes should always have a value")
	} else if match.Value != "" && match.Route == "" {
		t.Fatalf("values should always have a route")
	}
	actual := match.String()
	diff.TestString(t, actual, expect)
}

// Permute returns every permutation of the slice.
func permute(routes Routes) (out []Routes) {
	n := len(routes)

	// routes is too long, return the original
	// TODO: reduce the size of these tests over time
	if n > 10 {
		out = append(out, routes)
		return out
	}

	var backtrack func(int)
	backtrack = func(first int) {
		if first == n { // base‑case
			cp := slices.Clone(routes)
			out = append(out, cp)
			return
		}
		for i := first; i < n; i++ {
			routes[first], routes[i] = routes[i], routes[first] // choose
			backtrack(first + 1)                                // recurse ──► ✅ increment!
			routes[first], routes[i] = routes[i], routes[first] // un‑choose
		}
	}

	backtrack(0)
	return out
}

func matchEqual(t *testing.T, routes Routes) {
	t.Helper()
	// Test every combo of route order
	for _, routes := range permute(routes) {
		tree := enroute.New()
		for _, route := range routes {
			if err := tree.Insert(route.Route, "random"); err != nil {
				t.Fatal(err)
			}
			for _, request := range route.Requests {
				t.Run(route.Route, func(t *testing.T) {
					t.Helper()
					matchPath(t, tree, request.Path, request.Expect)
				})
			}
		}
	}
}

func matchExact(t *testing.T, routes Routes) {
	t.Helper()
	// Test every combo of route order
	tree := enroute.New()
	for _, route := range routes {
		if err := tree.Insert(route.Route, "random"); err != nil {
			t.Fatal(err)
		}
		for _, request := range route.Requests {
			t.Run(route.Route, func(t *testing.T) {
				t.Helper()
				matchPath(t, tree, request.Path, request.Expect)
			})
		}
	}
}

func TestSampleMatch(t *testing.T) {
	matchEqual(t, Routes{})
	matchEqual(t, Routes{
		{"/hello", Requests{
			{"/hello", `/hello`},
			{"/hello/world", `no match for "/hello/world"`},
			{"/", `no match for "/"`},
			{"/hello/", `/hello`},
		}},
	})
	matchEqual(t, Routes{
		{"/hello", Requests{
			{"/hello", `/hello`},
			{"/hello/world", `no match for "/hello/world"`},
			{"/hello/", `/hello`},
		}},
		{"/", Requests{
			{"/", `/`},
		}},
	})
	matchEqual(t, Routes{
		{"/v{version}", Requests{
			{"/v2", "/v{version} version=2"},
		}},
		{"/v{major}.{minor}.{patch}", Requests{
			{"/v2.0.1", "/v{major}.{minor}.{patch} major=2&minor=0&patch=1"},
		}},
		{"/v1", Requests{
			{"/v1", "/v1"},
		}},
		{"/v2.0.0", Requests{
			{"/v2.0.0", "/v2.0.0"},
		}},
	})
	matchEqual(t, Routes{
		{"/users/{id}/edit", Requests{
			{"/users/settings/edit", `/users/{id}/edit id=settings`},
		}},
		{"/users/settings", Requests{
			{"/users/settings", `/users/settings`},
		}},
		{"/v.{major}.{minor}", Requests{
			{"/v.1.0", `/v.{major}.{minor} major=1&minor=0`},
		}},
		{"/v.1", Requests{
			{"/v.1", `/v.1`},
		}},
	})
}

func TestNonRoutableNoMatch(t *testing.T) {
	is := is.New(t)
	tree := enroute.New()
	is.NoErr(tree.Insert("/hello", ""))
	is.NoErr(tree.Insert("/world", ""))
	match, err := tree.Match("/")
	is.True(errors.Is(err, enroute.ErrNoMatch))
	is.Equal(match, nil)
}

func TestNoRoutes(t *testing.T) {
	is := is.New(t)
	tree := enroute.New()
	match, err := tree.Match("/")
	is.True(errors.Is(err, enroute.ErrNoMatch))
	is.Equal(match, nil)
	match, err = tree.Match("/a")
	is.True(errors.Is(err, enroute.ErrNoMatch))
	is.Equal(match, nil)
	tree.Each(func(n *enroute.Node) bool {
		is.Fail() // should not be called
		return true
	})
	is.Equal(tree.String(), "")

}

func TestAllMatch(t *testing.T) {
	matchEqual(t, Routes{
		{"/hi", Requests{}},
		{"/ab", Requests{}},
		{"/about", Requests{}},
		{"/a", Requests{}},
		{"/α", Requests{}},
		{"/β", Requests{}},
		{"/users", Requests{}},
		{"/users/new", Requests{}},
		{"/users/id", Requests{}},
		{"/users/{id}", Requests{}},
		{"/users/{id}/edit", Requests{}},
		{"/posts/{post_id}/comments", Requests{}},
		{"/posts/{post_id}/comments/new", Requests{}},
		{"/posts/{post_id}/comments/{id}", Requests{}},
		{"/posts/{post_id}/comments/{id}/edit", Requests{}},
		{"/v.{version}", Requests{}},
		{"/v.{major}.{minor}.{patch}", Requests{}},
		{"/v.1", Requests{}},
		{"/v.2.0.0", Requests{}},
		{"/posts/{post_id}.{format}", Requests{}},
		{"/flights/{from}/{to}", Requests{}},
		{"/user/{user}/project/{project}", Requests{}},
		{"/archive/{year}/{month}", Requests{}},
		{"/search/{query}", Requests{
			{"/a", "/a"},
			{"/A", "/a"},
			{"/", `no match for "/"`},
			{"/hi", "/hi"},
			{"/about", "/about"},
			{"/ab", "/ab"},
			{"/abo", `no match for "/abo"`},   // key mismatch
			{"/abou", `no match for "/abou"`}, // key mismatch
			{"/no", `no match for "/no"`},     // no matching child
			{"/α", "/α"},
			{"/β", "/β"},
			{"/αβ", `no match for "/αβ"`},
			{"/users/id", "/users/id"},
			{"/users/10", "/users/{id} id=10"},
			{"/users/1", "/users/{id} id=1"},
			{"/users/a", "/users/{id} id=a"},
			{"/users/-", "/users/{id} id=-"},
			{"/users/_", "/users/{id} id=_"},
			{"/users/abc-d_e", "/users/{id} id=abc-d_e"},
			{"/users/10/edit", "/users/{id}/edit id=10"},
			{"/users/1/edit", "/users/{id}/edit id=1"},
			{"/users/a/edit", "/users/{id}/edit id=a"},
			{"/users/-/edit", "/users/{id}/edit id=-"},
			{"/users/_/edit", "/users/{id}/edit id=_"},
			{"/users/abc-d_e/edit", "/users/{id}/edit id=abc-d_e"},
			{"/posts/1/comments", "/posts/{post_id}/comments post_id=1"},
			{"/posts/10/comments", "/posts/{post_id}/comments post_id=10"},
			{"/posts/a/comments", "/posts/{post_id}/comments post_id=a"},
			{"/posts/-/comments", "/posts/{post_id}/comments post_id=-"},
			{"/posts/_/comments", "/posts/{post_id}/comments post_id=_"},
			{"/posts/abc-d_e/comments", "/posts/{post_id}/comments post_id=abc-d_e"},
			{"/posts/1/comments/2", "/posts/{post_id}/comments/{id} post_id=1&id=2"},
			{"/posts/10/comments/20", "/posts/{post_id}/comments/{id} post_id=10&id=20"},
			{"/posts/a/comments/b", "/posts/{post_id}/comments/{id} post_id=a&id=b"},
			{"/posts/-/comments/-", "/posts/{post_id}/comments/{id} post_id=-&id=-"},
			{"/posts/_/comments/_", "/posts/{post_id}/comments/{id} post_id=_&id=_"},
			{"/posts/abc-d_e/comments/x-y_z", "/posts/{post_id}/comments/{id} post_id=abc-d_e&id=x-y_z"},
			{"/posts/1/comments/2/edit", "/posts/{post_id}/comments/{id}/edit post_id=1&id=2"},
			{"/posts/10/comments/20/edit", "/posts/{post_id}/comments/{id}/edit post_id=10&id=20"},
			{"/posts/a/comments/b/edit", "/posts/{post_id}/comments/{id}/edit post_id=a&id=b"},
			{"/posts/-/comments/-/edit", "/posts/{post_id}/comments/{id}/edit post_id=-&id=-"},
			{"/posts/_/comments/_/edit", "/posts/{post_id}/comments/{id}/edit post_id=_&id=_"},
			{"/posts/abc-d_e/comments/x-y_z/edit", "/posts/{post_id}/comments/{id}/edit post_id=abc-d_e&id=x-y_z"},
			{"/v.1", "/v.1"},
			{"/v.2", "/v.{version} version=2"},
			{"/v.abc", "/v.{version} version=abc"},
			{"/v.2.0.0", "/v.2.0.0"},
			{"/posts/10.json", "/posts/{post_id}.{format} post_id=10&format=json"},
			{"/flights/Berlin/Madison", "/flights/{from}/{to} from=Berlin&to=Madison"},
			{"/archive/2021/2", "/archive/{year}/{month} year=2021&month=2"},
			{"/search/someth!ng+in+ünìcodé", "/search/{query} query=someth!ng+in+ünìcodé"},
			{"/search/with spaces", "/search/{query} query=with spaces"},
			{"/search/with/slashes", `no match for "/search/with/slashes"`},
		}},
	})
}

func TestMatchUnicode(t *testing.T) {
	matchEqual(t, Routes{
		{"/α", Requests{
			{"/α", `/α`},
		}},
		{"/β", Requests{
			{"/β", `/β`},
		}},
		{"/δ", Requests{
			{"/δ", `/δ`},
			{"/Δ", `/δ`},
			{"/αβ", `no match for "/αβ"`},
		}},
	})
}

func TestOptional(t *testing.T) {
	matchEqual(t, Routes{
		{"/{id?}", Requests{
			{"/", "/{id?}"},
			{"/10", "/{id?} id=10"},
			{"/a", "/{id?} id=a"},
			{"/users", "/{id?} id=users"},
			{"/users/", `/{id?} id=users`},
		}},
		{"/users/{id}.{format?}", Requests{
			{"/users/10", `no match for "/users/10"`},
			{"/users/10/", `no match for "/users/10"`},
			{"/users/10.", "/users/{id}.{format?} id=10"},
			{"/users/10.json", "/users/{id}.{format?} id=10&format=json"},
			{"/users/10.rss", "/users/{id}.{format?} id=10&format=rss"},
			{"/users/index.html", "/users/{id}.{format?} id=index&format=html"},
			{"/users/ü.html", "/users/{id}.{format?} id=ü&format=html"},
			{"/users/index.html/more", `no match for "/users/index.html/more"`},
		}},
		{"/users/v{version?}", Requests{
			{"/users/v10", "/users/v{version?} version=10"},
			{"/users/v1", "/users/v{version?} version=1"},
			{"/users/v", "/users/v{version?}"},
		}},
		{"/flights/{from}/{to?}", Requests{
			{"/flights/Berlin", "/flights/{from}/{to?} from=Berlin"},
			{"/flights/Berlin/", `/flights/{from}/{to?} from=Berlin`},
			{"/flights/Berlin/Madison", "/flights/{from}/{to?} from=Berlin&to=Madison"},
		}},
	})
}

func TestLastOptional(t *testing.T) {
	tree := enroute.New()
	insertEqual(t, tree, "/slash/{last?}/", `
		/slash [from=/slash/{last?}]
		••••••/{last} [from=/slash/{last?}]
	`)
	insertEqual(t, tree, "/not/{last?}/path", `optional slots must be at the end of the path`)
}

func TestWildcard(t *testing.T) {
	matchEqual(t, Routes{
		{"/{path*}", Requests{
			{"/", `/{path*}`},
			{"/10", `/{path*} path=10`},
			{"/10/20", `/{path*} path=10/20`},
			{"/api/v", `/{path*} path=api/v`},
		}},
		{"/users/{id}/{file*}", Requests{
			{"/users/10/dir/file.json", `/users/{id}/{file*} id=10&file=dir/file.json`},
			{"/users/10/dir", `/users/{id}/{file*} id=10&file=dir`},
			{"/users/10", `/users/{id}/{file*} id=10`},
		}},
		{"/api/v.{version*}", Requests{
			{"/api/v.2/1", `/api/v.{version*} version=2/1`},
			{"/api/v.2.1", `/api/v.{version*} version=2.1`},
			{"/api/v.", `/api/v.{version*}`},
		}},
	})
}

func TestLastWildcard(t *testing.T) {
	tree := enroute.New()
	insertEqual(t, tree, "/slash/{last*}/", `
		/slash [from=/slash/{last*}]
		••••••/{last*} [from=/slash/{last*}]
	`)
	insertEqual(t, tree, "/not/{last*}/path", `wildcard slots must be at the end of the path`)
}

func TestMatchDashedSlots(t *testing.T) {
	matchEqual(t, Routes{
		{"/{a}-{b}", Requests{
			{"/hello-world", `/{a}-{b} a=hello&b=world`},
			{"/a-b", `/{a}-{b} a=a&b=b`},
			{"/A-B", `/{a}-{b} a=A&b=B`},
			{"/AB", `no match for "/AB"`},
		}},
	})
}

func TestBackupTree(t *testing.T) {
	is := is.New(t)
	tree := enroute.New()
	insertEqual(t, tree, "/{post_id}/comments", `
		/{post_id}/comments [from=/{post_id}/comments]
	`)
	insertEqual(t, tree, "/{post_id}.{format}", `
		/{post_id}
		••••••••••/comments [from=/{post_id}/comments]
		••••••••••.{format} [from=/{post_id}.{format}]
	`)
	match, err := tree.Match("/10/comments")
	is.NoErr(err)
	is.Equal(match.String(), `/{post_id}/comments post_id=10`)
	match, err = tree.Match("/10.json")
	is.NoErr(err)
	is.Equal(match.String(), `/{post_id}.{format} post_id=10&format=json`)
}

func TestToRoutable(t *testing.T) {
	tree := enroute.New()
	insertEqual(t, tree, "/last", `
		/last [from=/last]
	`)
	insertEqual(t, tree, "/first", `
		/
		•last [from=/last]
		•first [from=/first]
	`)
	insertEqual(t, tree, "/{last*}", `
		/ [from=/{last*}]
		•last [from=/last]
		•first [from=/first]
		•{last*} [from=/{last*}]
	`)
}

func TestMatchRegexp(t *testing.T) {
	matchEqual(t, Routes{
		{"/{path|[A-Z]}", Requests{
			{"/A", `/{path|^[A-Z]$} path=A`},
			{"/B", `/{path|^[A-Z]$} path=B`},
			{"/Z", `/{path|^[A-Z]$} path=Z`},
			{"/AB", `no match for "/AB"`},
		}},
	})
	matchEqual(t, Routes{
		{"/{path|[A-Z]}", Requests{
			{"/A", `/{path|^[A-Z]$} path=A`},
		}},
		{"/{path|[0-9]}", Requests{
			{"/0", `/{path|^[0-9]$} path=0`},
			{"/9", `/{path|^[0-9]$} path=9`},
			{"/09", `no match for "/09"`},
		}},
	})
	matchEqual(t, Routes{
		{"/{path|[A-Z]}", Requests{
			{"/A", `/{path|^[A-Z]$} path=A`},
		}},
		{"/{path|[0-9]}", Requests{
			{"/0", `/{path|^[0-9]$} path=0`},
			{"/9", `/{path|^[0-9]$} path=9`},
			{"/09", `no match for "/09"`},
		}},
		{"/{path|[A-Z]{2,}}", Requests{
			{"/AB", `/{path|^[A-Z]{2,}$} path=AB`},
		}},
	})
	matchEqual(t, Routes{
		{"/{name}", Requests{
			{"/09", `/{name} name=09`},
		}},
		{"/{path|[A-Z]}", Requests{
			{"/A", `/{path|^[A-Z]$} path=A`},
		}},
		{"/{path|[0-9]}", Requests{
			{"/0", `/{path|^[0-9]$} path=0`},
			{"/9", `/{path|^[0-9]$} path=9`},
		}},
		{"/{path|[A-Z]{2,}}", Requests{
			{"/AB", `/{path|^[A-Z]{2,}$} path=AB`},
		}},
	})
	matchEqual(t, Routes{
		{"/{name}", Requests{
			{"/second", `/{name} name=second`},
			{"/09", `/{name} name=09`},
		}},
		{"/{path|[A-Z]}", Requests{
			{"/A", `/{path|^[A-Z]$} path=A`},
		}},
		{"/{path|[0-9]}", Requests{
			{"/0", `/{path|^[0-9]$} path=0`},
			{"/9", `/{path|^[0-9]$} path=9`},
		}},
		{"/first", Requests{
			{"/first", `/first`},
		}},
		{"/{path|[A-Z]{2,}}", Requests{
			{"/AB", `/{path|^[A-Z]{2,}$} path=AB`},
		}},
	})
	matchEqual(t, Routes{
		{"/v{version}", Requests{
			{"/v1", `/v{version} version=1`},
			{"/valpha.beta.omega", `/v{version} version=alpha.beta.omega`},
		}},
		{"/v{major|[0-9]}.{minor|[0-9]}", Requests{
			{"/v1.2", `/v{major|^[0-9]$}.{minor|^[0-9]$} major=1&minor=2`},
		}},
		{"/v{major|[0-9]}.{minor|[0-9]}.{patch|[0-9]}", Requests{
			{"/v1.2.3", `/v{major|^[0-9]$}.{minor|^[0-9]$}.{patch|^[0-9]$} major=1&minor=2&patch=3`},
		}},
	})
}

func TestResource(t *testing.T) {
	tree := enroute.New()
	insertEqual(t, tree, "/{id}/edit", `
		/{id}/edit [from=/{id}/edit]
	`)
	insertEqual(t, tree, "/", `
		/ [from=/]
		•{id}/edit [from=/{id}/edit]
	`)
	matchEqual(t, Routes{
		{"/{id}/edit", Requests{
			{"/2/edit", `/{id}/edit id=2`},
			{"/3/edit", `/{id}/edit id=3`},
		}},
		{"/", Requests{
			{"/", `/`},
		}},
	})
}

func TestFind(t *testing.T) {
	is := is.New(t)
	tree := enroute.New()
	a := "a"
	b := "b"
	is.NoErr(tree.Insert(`/{post_id}/comments`, a))
	is.NoErr(tree.Insert(`/{post_id}.{format}`, b))
	an, err := tree.Find(`/{post_id}/comments`)
	is.NoErr(err)
	is.Equal(an.Label, `/{post_id}/comments`)
	is.Equal(an.Value, a)
	bn, err := tree.Find(`/{post_id}.{format}`)
	is.NoErr(err)
	is.Equal(bn.Label, `/{post_id}.{format}`)
	is.Equal(bn.Value, b)
	cn, err := tree.Find(`/`)
	is.True(errors.Is(err, enroute.ErrNoMatch))
	is.Equal(cn, nil)
}

func TestFindByPrefix(t *testing.T) {
	is := is.New(t)
	tree := enroute.New()
	a := "a"
	is.NoErr(tree.Insert("/", a))
	node, err := tree.FindByPrefix("/{post_id}/comments")
	is.NoErr(err)
	is.Equal(node.Label, "/")
	node, err = tree.FindByPrefix("/{post_id}/")
	is.NoErr(err)
	is.Equal(node.Label, "/")
	node, err = tree.FindByPrefix("/{post_id}")
	is.NoErr(err)
	is.Equal(node.Label, "/")
	node, err = tree.FindByPrefix("/")
	is.NoErr(err)
	is.Equal(node.Label, "/")
	node, err = tree.FindByPrefix("/a")
	is.NoErr(err)
	is.Equal(node.Label, "/")

	// Add nested layout
	is.NoErr(tree.Insert("/{post_id}/comments", a))
	node, err = tree.FindByPrefix("/{post_id}/comments")
	is.NoErr(err)
	is.Equal(node.Label, "/{post_id}/comments")
	node, err = tree.FindByPrefix("/{post_id}/")
	is.NoErr(err)
	is.Equal(node.Label, "/")
	node, err = tree.FindByPrefix("/{post_id}")
	is.NoErr(err)
	is.Equal(node.Label, "/")
	node, err = tree.FindByPrefix("/")
	is.NoErr(err)
	is.Equal(node.Label, "/")

	// No root initially
	tree = enroute.New()
	is.NoErr(tree.Insert("/{post_id}/comments", a))
	node, err = tree.FindByPrefix("/{post_id}/comments")
	is.NoErr(err)
	is.Equal(node.Label, "/{post_id}/comments")
	node, err = tree.FindByPrefix("/{post_id}/")
	is.True(errors.Is(err, enroute.ErrNoMatch))
	is.Equal(node, nil)
	node, err = tree.FindByPrefix("/{post_id}")
	is.True(errors.Is(err, enroute.ErrNoMatch))
	is.Equal(node, nil)
	node, err = tree.FindByPrefix("/")
	is.True(errors.Is(err, enroute.ErrNoMatch))
	is.Equal(node, nil)
	node, err = tree.FindByPrefix("/a")
	is.True(errors.Is(err, enroute.ErrNoMatch))
	is.Equal(node, nil)
	// Add a subpath
	is.NoErr(tree.Insert("/{post_id}", a))
	node, err = tree.FindByPrefix("/{post_id}/comments")
	is.NoErr(err)
	is.Equal(node.Label, "/{post_id}/comments")
	node, err = tree.FindByPrefix("/{post_id}/")
	is.NoErr(err)
	is.Equal(node.Label, "/{post_id}")
	node, err = tree.FindByPrefix("/{post_id}")
	is.NoErr(err)
	is.Equal(node.Label, "/{post_id}")
	node, err = tree.FindByPrefix("/")
	is.True(errors.Is(err, enroute.ErrNoMatch))
	is.Equal(node, nil)
	node, err = tree.FindByPrefix("/a")
	is.True(errors.Is(err, enroute.ErrNoMatch))
	is.Equal(node, nil)
	// Add the root
	is.NoErr(tree.Insert("/", a))
	node, err = tree.FindByPrefix("/{post_id}/comments")
	is.NoErr(err)
	is.Equal(node.Label, "/{post_id}/comments")
	node, err = tree.FindByPrefix("/{post_id}/")
	is.NoErr(err)
	is.Equal(node.Label, "/{post_id}")
	node, err = tree.FindByPrefix("/{post_id}")
	is.NoErr(err)
	is.Equal(node.Label, "/{post_id}")
	node, err = tree.FindByPrefix("/")
	is.NoErr(err)
	is.Equal(node.Label, "/")
	node, err = tree.FindByPrefix("/a")
	is.NoErr(err)
	is.Equal(node.Label, "/")
}

func ExampleMatch() {
	matcher := enroute.New()
	matcher.Insert("/", "index.html")
	matcher.Insert("/users/{id}", "users/show.html")
	matcher.Insert("/posts/{post_id}/comments/{id}", "posts/comments/show.html")
	matcher.Insert("/fly/{from}-{to}", "fly/enroute.html")
	matcher.Insert("/v{major|[0-9]+}.{minor|[0-9]+}", "version.html")
	matcher.Insert("/{owner}/{repo}/{branch}/{path*}", "repo.html")
	match, err := matcher.Match("/matthewmueller/routes/main/internal/parser/parser.go")
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println(match.Path)
	fmt.Println(match.Route)
	fmt.Println(match.Value)
	if len(match.Slots) != 4 {
		fmt.Println("expected 4 slots, got ", len(match.Slots))
		return
	}
	fmt.Println(match.Slots[0].Key, match.Slots[0].Value)
	fmt.Println(match.Slots[1].Key, match.Slots[1].Value)
	fmt.Println(match.Slots[2].Key, match.Slots[2].Value)
	fmt.Println(match.Slots[3].Key, match.Slots[3].Value)

	// Output:
	// /matthewmueller/routes/main/internal/parser/parser.go
	// /{owner}/{repo}/{branch}/{path*}
	// repo.html
	// owner matthewmueller
	// repo routes
	// branch main
	// path internal/parser/parser.go
}

func TestParse(t *testing.T) {
	is := is.New(t)
	route, err := enroute.Parse("/posts/{post_id}/comments/{id}")
	is.NoErr(err)
	is.Equal(route.String(), "/posts/{post_id}/comments/{id}")
	route, err = enroute.Parse("posts/{post_id}/comments/{id}")
	is.True(err != nil)
	is.Equal(err.Error(), "path must start with a slash /")
	is.Equal(route, nil)
}

func TestMultipleSlots(t *testing.T) {
	tree := enroute.New()
	insertEqual(t, tree, "/border-spacing-{number}", `
		/border-spacing-{number} [from=/border-spacing-{number}]
	`)
	insertEqual(t, tree, "/border-spacing-x-{custom}", `
		/border-spacing-
		••••••••••••••••x-{custom} [from=/border-spacing-x-{custom}]
		••••••••••••••••{number} [from=/border-spacing-{number}]
	`)

	// Ensure reverse order produces the same results
	tree = enroute.New()
	insertEqual(t, tree, "/border-spacing-x-{custom}", `
		/border-spacing-x-{custom} [from=/border-spacing-x-{custom}]
	`)
	insertEqual(t, tree, "/border-spacing-{number}", `
		/border-spacing-
		••••••••••••••••x-{custom} [from=/border-spacing-x-{custom}]
		••••••••••••••••{number} [from=/border-spacing-{number}]
	`)
}

func TestMultipleSlashes(t *testing.T) {
	matchEqual(t, Routes{
		{"/", Requests{
			{"/", `/`},
			{"//", `/`},
			{"///", `/`},
		}},
	})
}

func TestExisting(t *testing.T) {
	tree := enroute.New()
	insertEqual(t, tree, "/{path*}", `
		/ [from=/{path*}]
		•{path*} [from=/{path*}]
	`)
	insertEqual(t, tree, "/", `
		/ [from=/]
		•{path*} [from=/{path*}]
	`)
}

func TestMatchPrecedence(t *testing.T) {
	matchExact(t, Routes{
		{"/", Requests{
			{"/", `/`},
		}},
		{"/{digits|[0-9]+}", Requests{
			{"/10", `/{digits|^[0-9]+$} digits=10`},
			{"/20", `/{digits|^[0-9]+$} digits=20`},
			{"/2", `/{digits|^[0-9]+$} digits=2`},
		}},
		{"/{public?}", Requests{
			{"/a", `/{public?} public=a`},
			{"/a/", `/{public?} public=a`},
			{"/A", `/{public?} public=A`},
			{"/α", `/{public?} public=α`},
		}},
		{"/{public*}", Requests{
			{"/a/b", `/{public*} public=a/b`},
			{"/a/b/", `/{public*} public=a/b`},
			{"/a/b/c", `/{public*} public=a/b/c`},
			{"/α/β/γ", `/{public*} public=α/β/γ`},
			{"/a/b/c/", `/{public*} public=a/b/c`},
			{"/a/b/c/d", `/{public*} public=a/b/c/d`},
			{"/a/b/c/d/", `/{public*} public=a/b/c/d`},
		}},
	})

	matchEqual(t, Routes{
		{"/", Requests{
			{"/", `/`},
		}},
		{"/{digits|[0-9]+}", Requests{
			{"/10", `/{digits|^[0-9]+$} digits=10`},
			{"/20", `/{digits|^[0-9]+$} digits=20`},
			{"/2", `/{digits|^[0-9]+$} digits=2`},
		}},
		{"/{public*}", Requests{
			{"/a", `/{public*} public=a`},
			{"/a/", `/{public*} public=a`},
			{"/A", `/{public*} public=A`},
			{"/α", `/{public*} public=α`},
			{"/a/b", `/{public*} public=a/b`},
			{"/a/b/", `/{public*} public=a/b`},
			{"/a/b/c", `/{public*} public=a/b/c`},
			{"/α/β/γ", `/{public*} public=α/β/γ`},
			{"/a/b/c/", `/{public*} public=a/b/c`},
			{"/a/b/c/d", `/{public*} public=a/b/c/d`},
			{"/a/b/c/d/", `/{public*} public=a/b/c/d`},
		}},
	})

	matchEqual(t, Routes{
		{"/", Requests{
			{"/", `/`},
		}},
		{"/{digits|[0-9]+}", Requests{
			{"/10", `/{digits|^[0-9]+$} digits=10`},
			{"/20", `/{digits|^[0-9]+$} digits=20`},
			{"/2", `/{digits|^[0-9]+$} digits=2`},
		}},
		{"/{public?}", Requests{
			{"/a", `/{public?} public=a`},
			{"/a/", `/{public?} public=a`},
			{"/A", `/{public?} public=A`},
			{"/α", `/{public?} public=α`},
		}},
	})
}
