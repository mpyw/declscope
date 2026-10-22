package internal

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"strings"
	"sync"
)

// folderListing is what the process knows of one source folder: its .go
// files, and the package clause and names of each file read so far. unseen
// asks it once per pass, and the non-test, test and external test variants of
// a package ask about the same folder, so the listing and every file read are
// shared by the passes of the process rather than redone by each.
//
// It is kept while the folder keeps its modification time and size, as a
// loaded baseline is, and for the same reason: the analyzer may live in a
// process that outlasts many edits (gopls, golangci-lint's cache server), and
// cannot tell when a run starts. Adding, removing or renaming a file changes
// the folder's modification time, and the folder is then listed again. A file
// is kept while it keeps its own modification time and size.
type folderListing struct {
	stamp folderStamp
	once  sync.Once
	err   error

	// goFiles is the name of every entry ending in .go that is not a
	// directory, in the order os.ReadDir returns them.
	//
	//declscope:shared // unseen walks it
	goFiles []string

	mu    sync.Mutex
	files map[string]*folderFile
}

// folderFile is what one .go file of a listed folder declares and names.
type folderFile struct {
	stamp folderStamp

	clauseOnce sync.Once
	clauseErr  error
	// pkg is the name in the file's package clause.
	//
	//declscope:shared // unseen skips a file of another package on it
	pkg string

	namesOnce sync.Once
	namesErr  error
	// names is every identifier the file writes. It is read only for a file
	// of the package under analysis, so a generator of another package is
	// never parsed past its package clause.
	names []string
}

// folderStamp is the state a listing or a file read is kept under.
type folderStamp struct {
	modTime int64
	size    int64
}

var (
	folderMu       sync.Mutex
	folderListings = map[string]*folderListing{}
)

// listFolder returns the .go files of dir, listed once per state of dir.
//
// A stat rather than a listing on every call: a hit costs one system call. A
// file added between the stat and the listing is kept under the older state,
// which the next stat no longer matches, so the folder is listed again rather
// than the file missed. An error is not kept, so the next call tries again.
//
//declscope:shared // unseen lists the source folder through it
func listFolder(dir string) (*folderListing, error) {
	st, err := os.Stat(dir)
	if err != nil {
		folderMu.Lock()
		delete(folderListings, dir)
		folderMu.Unlock()
		return nil, err
	}
	stamp := folderStampOf(st)

	folderMu.Lock()
	l := folderListings[dir]
	if l == nil || l.stamp != stamp {
		l = &folderListing{stamp: stamp, files: map[string]*folderFile{}}
		folderListings[dir] = l
	}
	folderMu.Unlock()

	l.once.Do(func() {
		entries, err := os.ReadDir(dir)
		if err != nil {
			l.err = err
			folderMu.Lock()
			if folderListings[dir] == l {
				delete(folderListings, dir)
			}
			folderMu.Unlock()
			return
		}
		for _, e := range entries {
			if !e.IsDir() && strings.HasSuffix(e.Name(), ".go") {
				l.goFiles = append(l.goFiles, e.Name())
			}
		}
	})
	return l, l.err
}

// readFolderFile returns the file at path, which the listing holds, read once
// per state of the file. Its package clause is read on the first call, and the
// rest only when readFolderFileNames asks.
//
//declscope:shared // unseen reads every file it does not see through it
func (l *folderListing) readFolderFile(path string) (*folderFile, error) {
	st, err := os.Stat(path)
	if err != nil {
		l.dropFolderFile(path, nil)
		return nil, err
	}
	stamp := folderStampOf(st)

	l.mu.Lock()
	f := l.files[path]
	if f == nil || f.stamp != stamp {
		f = &folderFile{stamp: stamp}
		l.files[path] = f
	}
	l.mu.Unlock()

	f.clauseOnce.Do(func() {
		var file *ast.File
		if file, f.clauseErr = parser.ParseFile(token.NewFileSet(), path, nil, parser.PackageClauseOnly); f.clauseErr == nil {
			f.pkg = file.Name.Name
		}
	})
	if f.clauseErr != nil {
		l.dropFolderFile(path, f)
		return nil, f.clauseErr
	}
	return f, nil
}

// readFolderFileNames returns every identifier the file at path writes. The
// file is parsed in full once per state, and only the names are kept.
//
//declscope:shared // unseen reads the names of each file of its package
func (l *folderListing) readFolderFileNames(path string, f *folderFile) ([]string, error) {
	f.namesOnce.Do(func() {
		// Only the names are read, so the parser need not resolve them.
		file, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.SkipObjectResolution)
		if err != nil {
			f.namesErr = err
			return
		}
		seen := map[string]bool{}
		ast.Inspect(file, func(n ast.Node) bool {
			if id, ok := n.(*ast.Ident); ok && !seen[id.Name] {
				seen[id.Name] = true
				f.names = append(f.names, id.Name)
			}
			return true
		})
	})
	if f.namesErr != nil {
		l.dropFolderFile(path, f)
		return nil, f.namesErr
	}
	return f.names, nil
}

// dropFolderFile forgets a file that failed to read, so the next call reads it
// again: a chmod or a passing shortage of file descriptors changes neither the
// modification time nor the size. The calls already waiting on that read share
// the error. A newer read is left alone; with f nil, any read is dropped.
func (l *folderListing) dropFolderFile(path string, f *folderFile) {
	l.mu.Lock()
	if f == nil || l.files[path] == f {
		delete(l.files, path)
	}
	l.mu.Unlock()
}

// folderStampOf is the state a stat reports.
func folderStampOf(st os.FileInfo) folderStamp {
	return folderStamp{modTime: st.ModTime().UnixNano(), size: st.Size()}
}
