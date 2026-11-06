package kubernetes

import (
	"bytes"
	"io"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBufferedPipe_WriteDoesNotWaitForSlowReader(t *testing.T) {
	data := bytes.Repeat([]byte("k8up"), 512*1024) // 2 MiB
	p := newBufferedPipe(4 * 1024 * 1024)

	written := make(chan error, 1)
	go func() {
		_, err := p.Write(data)
		written <- err
		_ = p.CloseWrite()
	}()

	// Nobody reads yet, like restic while it opens the repository.
	select {
	case err := <-written:
		require.NoError(t, err)
	case <-time.After(2 * time.Second):
		t.Fatal("Write blocked although the buffer had room and the reader had not started")
	}

	got, err := io.ReadAll(p)
	require.NoError(t, err)
	assert.Equal(t, data, got)
}

func TestBufferedPipe_WriteBlocksWhenFullAndResumes(t *testing.T) {
	data := bytes.Repeat([]byte("x"), 1000)
	p := newBufferedPipe(100)

	done := make(chan error, 1)
	go func() {
		_, err := p.Write(data)
		_ = p.CloseWrite()
		done <- err
	}()

	select {
	case err := <-done:
		t.Fatalf("Write finished (err: %v) although the buffer is smaller than the data", err)
	case <-time.After(100 * time.Millisecond):
	}

	got, err := io.ReadAll(p)
	require.NoError(t, err)
	assert.Equal(t, data, got)
	require.NoError(t, <-done)
}

func TestBufferedPipe_CloseUnblocksWriter(t *testing.T) {
	p := newBufferedPipe(10)

	done := make(chan error, 1)
	go func() {
		_, err := p.Write(make([]byte, 100))
		done <- err
	}()

	time.Sleep(50 * time.Millisecond)
	require.NoError(t, p.Close())

	select {
	case err := <-done:
		assert.ErrorIs(t, err, io.ErrClosedPipe)
	case <-time.After(2 * time.Second):
		t.Fatal("Write is still blocked after the reader closed")
	}
}

func TestBufferedPipe_EOFOnlyAfterQueuedDataIsRead(t *testing.T) {
	p := newBufferedPipe(100)
	_, err := p.Write([]byte("abc"))
	require.NoError(t, err)
	require.NoError(t, p.CloseWrite())

	buf := make([]byte, 2)
	n, err := p.Read(buf)
	require.NoError(t, err)
	assert.Equal(t, "ab", string(buf[:n]))
	n, err = p.Read(buf)
	require.NoError(t, err)
	assert.Equal(t, "c", string(buf[:n]))
	_, err = p.Read(buf)
	assert.Equal(t, io.EOF, err)
}
