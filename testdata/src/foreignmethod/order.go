package foreignmethod

// A foreign method: userRecord is declared in user.go, so the receiver names a
// unit that does not hold this method. The rule asks for order.
func (u *userRecord) leak() string { // want `method leak does not carry namespace "order" anywhere in its name; move it to the file it belongs to, or give it a name that carries "order" \(orderLeak is one candidate, not necessarily the best\)`
	return u.name
}

// Already carrying the namespace: nothing is asked.
func (u *userRecord) orderDump() string { return u.name }

func orderRun(s userSealed) string {
	u := userMake()
	return u.bump() + u.leak() + u.orderDump()
}
