package conn

// The baseline records Conn.Close. Unexported, Conn would change that key to
// conn.Close, and the next run would report Close, so the fix is withheld.
type Conn struct{} // want: type Conn is exported, but nothing outside example.com/baselinerename/internal/conn uses it \(no fix: the baseline records a member of it\)$

// The baseline records Close itself, so it is not reported.
func (Conn) Close() error { return nil }

// The baseline records Open under the analyzer's naming rule. Unexported, the
// entry would match nothing, so the fix is withheld here too.
func Open() {} // want: func Open is exported, but nothing outside example.com/baselinerename/internal/conn uses it \(no fix: the baseline records it\)$

// The baseline records only a name nothing here spells, so Fresh is fixed.
func Fresh() {} // want: func Fresh is exported, but nothing outside example.com/baselinerename/internal/conn uses it$
