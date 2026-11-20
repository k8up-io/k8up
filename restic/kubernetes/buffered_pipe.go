package kubernetes

import (
	"bytes"
	"io"
	"sync"
)

// execBufferSize is how much command output can wait in memory before the
// exec stream is held back. restic needs a few seconds to open the repository
// before it reads stdin, and the Kubernetes exec stream is dropped if nobody
// reads it for about 5 seconds.
const execBufferSize = 64 * 1024 * 1024

// bufferedPipe is like io.Pipe, but Write only blocks once `max` bytes are
// waiting to be read. With io.Pipe every Write blocks until a Read takes the
// data, so a slow reader stalls the writer right away.
type bufferedPipe struct {
	mu           sync.Mutex
	cond         *sync.Cond
	buf          bytes.Buffer
	max          int
	writerClosed bool
	readerClosed bool
}

func newBufferedPipe(max int) *bufferedPipe {
	p := &bufferedPipe{max: max}
	p.cond = sync.NewCond(&p.mu)
	return p
}

// Write queues data for the reader. It blocks while the buffer is full.
func (p *bufferedPipe) Write(data []byte) (int, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	written := 0
	for len(data) > 0 {
		for p.buf.Len() >= p.max && !p.readerClosed && !p.writerClosed {
			p.cond.Wait()
		}
		if p.readerClosed || p.writerClosed {
			return written, io.ErrClosedPipe
		}
		n := min(len(data), p.max-p.buf.Len())
		p.buf.Write(data[:n])
		data = data[n:]
		written += n
		p.cond.Broadcast()
	}
	return written, nil
}

// Read returns queued data, blocks while there is none, and returns io.EOF
// after CloseWrite once everything has been read.
func (p *bufferedPipe) Read(out []byte) (int, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	for p.buf.Len() == 0 && !p.writerClosed && !p.readerClosed {
		p.cond.Wait()
	}
	if p.readerClosed {
		return 0, io.ErrClosedPipe
	}
	if p.buf.Len() == 0 {
		return 0, io.EOF
	}
	n, _ := p.buf.Read(out)
	p.cond.Broadcast()
	return n, nil
}

// CloseWrite signals that no more data will be written. The reader still gets
// the data that is already queued.
func (p *bufferedPipe) CloseWrite() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.writerClosed = true
	p.cond.Broadcast()
	return nil
}

// Close stops the reader side. Pending and future Writes fail.
func (p *bufferedPipe) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.readerClosed = true
	p.buf.Reset()
	p.cond.Broadcast()
	return nil
}
