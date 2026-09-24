package internal

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
)

// A filter pattern answers "which files on disk", so every question this
// file asks is about paths rather than about names. The config file that
// states a pattern is what "here" means for it, and the file being analysed
// arrives spelled however the driver spelled it — through a symlink or
// through what the symlink points at. Both facts live here so that Options
// only has to ask whether a path is in or out.

// filterMatcher decides one pattern. An anchored matcher holds the
// directories its pattern is relative to and matches against the path taken
// from one of them; a floating matcher has no bases and matches the path as
// given, at any depth.
//
//declscope:package
type filterMatcher struct {
	//declscope:private
	re *regexp.Regexp
	//declscope:private
	bases []string
}

// filterBases returns the spellings of dir that a reported path may use.
// EvalSymlinks gives the second one: a driver handed /tmp on macOS reports
// paths under /tmp while a config discovered by walking up may be named
// /private/tmp, and a pattern anchored to one spelling must still recognise
// the other. Anchoring on text alone would silently exclude nothing.
//
//declscope:package
func filterBases(dir string) []string {
	if dir == "" {
		return nil
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		abs = dir
	}
	bases := []string{abs}
	if real, err := filepath.EvalSymlinks(abs); err == nil && real != abs {
		bases = append(bases, real)
	}
	return bases
}

// compileFilter translates one pattern into a matcher. only and omit share
// it: a pattern means the same thing whichever list holds it. ** matches across
// separators, * and ? do not.
//
// Whether the pattern anchors follows .gitignore, because that is the file
// every reader already has an intuition for:
//
//	gen.go        floats: a file of that name at any depth
//	**/gen/**     floats: that directory at any depth
//	gen/**        anchors: the one beside this config file
//	/gen/**       anchors: the same, spelled the way .gitignore anchors
//
// Anchoring at load time, by joining the pattern onto the config directory as
// text, would look equivalent and is not: a config named through a symlink,
// through a relative path, or from outside the tree produces a pattern that
// matches nothing at all, with no error to say so. The join happens here, per
// path, against every spelling the directory has.
//
//declscope:package
func compileFilter(pattern string, bases []string) (filterMatcher, error) {
	p := filepath.ToSlash(pattern)
	for strings.HasPrefix(p, "./") {
		// "./vendor" is one of the ways a person writes "the vendor beside
		// this file". It anchors like "vendor/**" rather than naming a
		// directory called ".".
		p = p[2:]
	}
	for _, seg := range strings.Split(p, "/") {
		if seg == ".." {
			// A pattern is matched against a path taken from the config
			// directory, which never contains a "..". Accepting one would
			// mean accepting a pattern that can never match anything.
			return filterMatcher{}, fmt.Errorf("filter %q: a pattern cannot leave the directory of the config file that states it", pattern)
		}
	}
	floats := strings.HasPrefix(p, "**/") || !strings.Contains(strings.TrimSuffix(p, "/"), "/")
	if strings.HasPrefix(p, "/") {
		// A leading separator is .gitignore's way of saying "here, not at any
		// depth". Reading it as a filesystem-absolute path instead would make
		// the same config mean different things on different machines, and on
		// Windows filepath.IsAbs would not even agree that it was one.
		p = strings.TrimLeft(p, "/")
		floats = false
	}
	if len(bases) == 0 {
		// No config file said where "here" is, so there is nothing to anchor
		// to and every pattern floats.
		floats = true
	}

	var b strings.Builder
	if floats {
		b.WriteString("(?:^|/)")
	} else {
		b.WriteString("^")
	}
	for i := 0; i < len(p); {
		switch {
		case strings.HasPrefix(p[i:], "**/"):
			b.WriteString("(?:.*/)?")
			i += 3
		case strings.HasPrefix(p[i:], "**"):
			b.WriteString(".*")
			i += 2
		case p[i] == '*':
			b.WriteString("[^/]*")
			i++
		case p[i] == '?':
			b.WriteString("[^/]")
			i++
		default:
			b.WriteString(regexp.QuoteMeta(p[i : i+1]))
			i++
		}
	}
	b.WriteString("$")

	re, err := regexp.Compile(b.String())
	if err != nil {
		return filterMatcher{}, err
	}
	if floats {
		return filterMatcher{re: re}, nil
	}
	return filterMatcher{re: re, bases: bases}, nil
}

// filterMatches reports whether any matcher covers path.
//
// A path that no anchored matcher recognises is tried again with its symlinks
// resolved. The driver reports whichever spelling it was given, and an
// explicit -config may be spelled the other way; resolving the config
// directory alone only covers one of the two directions. The resolution is
// done once per file and only when an anchored pattern exists, so a config
// without one costs nothing.
//
//declscope:package
func filterMatches(ms []filterMatcher, path string) bool {
	resolved, tried := "", false
	for _, m := range ms {
		if m.match(path) {
			return true
		}
		if len(m.bases) == 0 {
			continue
		}
		if !tried {
			tried = true
			if r, err := filepath.EvalSymlinks(path); err == nil && r != path {
				resolved = r
			}
		}
		if resolved != "" && m.match(resolved) {
			return true
		}
	}
	return false
}

// match reports whether the pattern covers path.
func (m filterMatcher) match(path string) bool {
	if len(m.bases) == 0 {
		return m.re.MatchString(filepath.ToSlash(path))
	}
	for _, base := range m.bases {
		rel, err := filepath.Rel(base, path)
		if err != nil {
			// Rel refuses to compare a relative path with an absolute one.
			// The other spelling may still work, so this is not a verdict.
			continue
		}
		rel = filepath.ToSlash(rel)
		if rel == ".." || strings.HasPrefix(rel, "../") {
			// Outside the tree the config file speaks for.
			continue
		}
		if m.re.MatchString(rel) {
			return true
		}
	}
	return false
}

// FilterPattern is a pattern together with the directory it was written in.
// The two travel as a pair because a chain of config files states patterns at
// several depths, and each anchors to its own: "gen/**" in the root and
// "gen/**" in a nested file name different directories.
//
// Base is empty for a pattern from options no config file produced, which
// leaves it floating, since there is no directory to call "here".
type FilterPattern struct {
	Pattern string
	Base    string
}
