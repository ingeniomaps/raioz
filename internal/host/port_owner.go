package host

// ProcessGroupListensOn reports whether a process in the group led by pgid
// holds a listening TCP socket on port. raioz starts every host service as
// the leader of its own group, so this answers "is that port ours?" for a
// recorded service PID.
//
// known is false where the answer cannot be worked out (non-Linux, or
// /proc unreadable); callers then decide on the weaker evidence they have.
func ProcessGroupListensOn(pgid, port int) (held, known bool) {
	if pgid <= 0 || port <= 0 {
		return false, true
	}
	return processGroupListensOn(pgid, port)
}

// GroupListeningPorts returns the TCP ports the process group led by pgid
// listens on, sorted ascending. known is false where it cannot be worked
// out (non-Linux, or /proc unreadable).
func GroupListeningPorts(pgid int) (ports []int, known bool) {
	if pgid <= 0 {
		return nil, true
	}
	return groupListeningPorts(pgid)
}
