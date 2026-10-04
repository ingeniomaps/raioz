//go:build linux

package host

import (
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// tcpListenState is the `st` column of /proc/net/tcp for a LISTEN socket.
const tcpListenState = "0A"

func processGroupListensOn(pgid, port int) (held, known bool) {
	inodes, ok := listeningInodes(port)
	if !ok {
		return false, false
	}
	if len(inodes) == 0 {
		return false, true
	}
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return false, false
	}
	for _, e := range entries {
		pid, err := strconv.Atoi(e.Name())
		if err != nil || processGroup(pid) != pgid {
			continue
		}
		if holdsSocket(pid, inodes) {
			return true, true
		}
	}
	return false, true
}

// listeningInodes returns the socket inodes listening on port, from both
// address families. ok is false when neither table could be read.
func listeningInodes(port int) (map[string]bool, bool) {
	all, ok := listeningSockets()
	inodes := map[string]bool{}
	for inode, p := range all {
		if p == port {
			inodes[inode] = true
		}
	}
	return inodes, ok
}

// listeningSockets maps the inode of every listening TCP socket to its
// port, from both address families. ok is false when neither table could
// be read.
func listeningSockets() (map[string]int, bool) {
	sockets := map[string]int{}
	read := false
	for _, table := range []string{"/proc/net/tcp", "/proc/net/tcp6"} {
		data, err := os.ReadFile(table)
		if err != nil {
			continue
		}
		read = true
		for _, line := range strings.Split(string(data), "\n") {
			// sl local_address rem_address st tx_queue rx_queue tr tm->when
			// retrnsmt uid timeout inode ...
			f := strings.Fields(line)
			if len(f) < 10 || f[3] != tcpListenState {
				continue
			}
			i := strings.LastIndexByte(f[1], ':')
			if i < 0 {
				continue
			}
			if p, err := strconv.ParseInt(f[1][i+1:], 16, 32); err == nil {
				sockets[f[9]] = int(p)
			}
		}
	}
	return sockets, read
}

func groupListeningPorts(pgid int) ([]int, bool) {
	sockets, ok := listeningSockets()
	if !ok {
		return nil, false
	}
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return nil, false
	}
	seen := map[int]bool{}
	for _, e := range entries {
		pid, err := strconv.Atoi(e.Name())
		if err != nil || processGroup(pid) != pgid {
			continue
		}
		for _, inode := range socketInodes(pid) {
			if port, listening := sockets[inode]; listening {
				seen[port] = true
			}
		}
	}
	ports := make([]int, 0, len(seen))
	for port := range seen {
		ports = append(ports, port)
	}
	sort.Ints(ports)
	return ports, true
}

// processGroup reads the pgrp of pid from /proc/<pid>/stat, 0 when it is
// gone or unreadable. Anchors on the last ')' like parentPID: the comm
// field may itself contain spaces and parentheses.
func processGroup(pid int) int {
	data, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/stat")
	if err != nil {
		return 0
	}
	rest := string(data)
	idx := strings.LastIndexByte(rest, ')')
	if idx < 0 || idx+2 >= len(rest) {
		return 0
	}
	fields := strings.Fields(rest[idx+2:]) // state ppid pgrp ...
	if len(fields) < 3 {
		return 0
	}
	pgrp, err := strconv.Atoi(fields[2])
	if err != nil {
		return 0
	}
	return pgrp
}

func holdsSocket(pid int, inodes map[string]bool) bool {
	for _, inode := range socketInodes(pid) {
		if inodes[inode] {
			return true
		}
	}
	return false
}

// socketInodes returns the inode of every socket pid has open.
func socketInodes(pid int) []string {
	dir := "/proc/" + strconv.Itoa(pid) + "/fd"
	fds, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var inodes []string
	for _, fd := range fds {
		target, err := os.Readlink(dir + "/" + fd.Name())
		if err != nil {
			continue
		}
		if inode, ok := strings.CutPrefix(target, "socket:["); ok {
			inodes = append(inodes, strings.TrimSuffix(inode, "]"))
		}
	}
	return inodes
}

func processRunsIn(pid int, dir string) (within, known bool) {
	cwd, err := os.Readlink("/proc/" + strconv.Itoa(pid) + "/cwd")
	if err != nil {
		return false, false
	}
	clean := filepath.Clean(dir)
	if resolved, err := filepath.EvalSymlinks(clean); err == nil {
		clean = resolved
	}
	return cwd == clean || strings.HasPrefix(cwd, clean+string(filepath.Separator)), true
}
