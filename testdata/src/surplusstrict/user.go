package surplusstrict

// userAccount is shared: order.go spells it and reads id. balance is read
// only here, so the type's directive widens it for nothing.
//
//declscope:shared
type userAccount struct {
	id      int
	balance int // want `field userAccount.balance takes shared scope from //declscope:shared on userAccount, but no use from another namespace is visible to declscope`

	// Name is exported, so it is shared whatever the type says.
	Name string

	// owner states its own scope, so the type's directive does not reach it.
	//
	//declscope:private
	owner string

	// An embedded field has no name of its own, and is never a target.
	*userLedger
}

type userLedger struct{ entries int }

// One entry holds two names, and one directive above it would bind both. Both
// are read only here, so both are reported, and one fix narrows the two.
//
//declscope:shared
type userPair struct {
	left, right int // want `field userPair.left takes shared scope` `field userPair.right takes shared scope`
	shared      int
}

// top is read from order.go and bottom is not. Only a directive above the
// whole entry could narrow bottom, and it would narrow top as well, so the
// entry is not reported.
//
//declscope:shared
type userMixed struct {
	top, bottom int
}

// The type's directive is surplus: nothing about userIdle crosses. surplus
// reports the directive, and the fields are not reported again under it.
//
//declscope:shared // want `//declscope:shared on userIdle, userIdle.count: no use from another namespace is visible to declscope`
type userIdle struct {
	count int
}

// An ignore silences the rule on a field, and on every field of a type.
//
//declscope:shared
type userQuiet struct {
	//declscope:ignore surplus
	hush int
	kept int // want `field userQuiet.kept takes shared scope`
}

//declscope:shared
//declscope:ignore surplus
type userHushed struct {
	a int
	b int
}

// An ignore for this rule on a field that it would not report is unused.
//
//declscope:shared
type userNeedless struct {
	//declscope:ignore surplus // want `unused //declscope:ignore surplus on userNeedless.read`
	read int
}

// An interface's method names are members too, and are judged like fields.
// Satisfying the interface is not a use of the name, so an implementation in
// another namespace would not keep save wide; a call from there would.
//
//declscope:shared
type userSaver interface {
	save() int // want `method userSaver.save takes shared scope from //declscope:shared on userSaver`
}

func userLocal() int {
	a := userAccount{balance: 1, owner: "o"}
	p := userPair{left: 1, right: 2}
	q := userQuiet{hush: 1, kept: 2}
	h := userHushed{a: 1, b: 2}
	var s userSaver
	_ = s
	return a.balance + p.left + p.right + q.hush + q.kept + h.a + h.b + userIdle{}.count + len(a.owner)
}

var _ = userLocal

// A directive on a block reaches every spec, and strict judges each one.
//
//declscope:shared
var (
	blockUsed  = 1
	blockLocal = 2 // want `var blockLocal takes shared scope from the //declscope:shared on its block, but no use from another namespace is visible to declscope`
)

var _ = blockLocal
