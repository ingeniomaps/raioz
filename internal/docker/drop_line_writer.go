package docker

import (
	"bytes"
	"io"
)

// dropLineWriter forwards what is written to it line by line, leaving out
// the lines that contain drop. It lets a command stream its progress to the
// terminal minus one message that is known to be noise.
type dropLineWriter struct {
	w    io.Writer
	drop string
	buf  []byte
}

// Write buffers until a line is complete, then forwards or drops it.
func (d *dropLineWriter) Write(p []byte) (int, error) {
	d.buf = append(d.buf, p...)
	for {
		i := bytes.IndexByte(d.buf, '\n')
		if i < 0 {
			return len(p), nil
		}
		line := d.buf[:i+1]
		d.buf = d.buf[i+1:]
		if bytes.Contains(line, []byte(d.drop)) {
			continue
		}
		if _, err := d.w.Write(line); err != nil {
			return len(p), err //nolint:wrapcheck // passthrough writer
		}
	}
}

// Flush forwards a trailing line that never got its newline.
func (d *dropLineWriter) Flush() {
	if len(d.buf) > 0 && !bytes.Contains(d.buf, []byte(d.drop)) {
		_, _ = d.w.Write(d.buf)
	}
	d.buf = nil
}
