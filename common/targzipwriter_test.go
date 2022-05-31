package common_test

import (
	"archive/tar"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/k8up-io/k8up/v2/common"
)

type MockWriter struct {
	WriteError error
	Buffer     []byte
}

var (
	testTarHeader *tar.Header
	testData      []byte
)

func init() {
	testData = []byte{0x00, 0xFF, 0x00, 0xFF, 0x00, 0xFF, 0x00, 0xFF, 0x00, 0xFF, 0x00, 0xFF, 0x00, 0xFF, 0x00, 0xFF, 0x00, 0xFF, 0x00, 0xFF}
	testTarHeader = &tar.Header{
		Name: "testName",
		Size: int64(len(testData)),
	}
}

func (w *MockWriter) Write(p []byte) (int, error) {
	if w.Buffer == nil {
		w.Buffer = make([]byte, len(p))
	}

	before := len(w.Buffer)
	w.Buffer = append(w.Buffer, p...)
	written := len(w.Buffer) - before
	return written, w.WriteError
}

func (w *MockWriter) Close() error {
	return fmt.Errorf("close must not be called")
}

func Test_NewTarGzipWriter(t *testing.T) {
	tgz := common.NewTarGzipWriter(&MockWriter{})
	assert.NotNil(t, tgz)
}

func Test_WriteHeader(t *testing.T) {
	tgz := common.NewTarGzipWriter(&MockWriter{})
	require.NotNil(t, tgz)

	err := tgz.WriteHeader(testTarHeader)
	assert.NoError(t, err)
}

func Test_WriteHeaderErr(t *testing.T) {
	mock := &MockWriter{}
	mock.WriteError = fmt.Errorf("test error")
	tgz := common.NewTarGzipWriter(mock)
	require.NotNil(t, tgz)

	err := tgz.WriteHeader(testTarHeader)
	assert.Error(t, err)
}

func Test_Write(t *testing.T) {
	mock := &MockWriter{}
	tgz := common.NewTarGzipWriter(mock)
	require.NotNil(t, tgz)

	err := tgz.WriteHeader(testTarHeader)
	require.NoError(t, err)

	lenAfterHeader := len(mock.Buffer)
	assert.Greater(t, lenAfterHeader, 0)

	n, err := tgz.Write(testData)
	assert.NoError(t, err)
	assert.Equal(t, len(testData), n)

	err = tgz.Close() // Flush
	require.NoError(t, err)

	lenAfterData := len(mock.Buffer)
	assert.Greater(t, lenAfterData, lenAfterHeader)
}

func Test_WriteErr(t *testing.T) {
	mock := &MockWriter{}
	tgz := common.NewTarGzipWriter(mock)
	require.NotNil(t, tgz)

	err := tgz.WriteHeader(testTarHeader)
	require.NoError(t, err)

	mock.WriteError = fmt.Errorf("test error")

	_, writeErr := tgz.Write(testData)

	closeErr := tgz.Close() // Flush

	// Either closeErr or writeErr should error
	if writeErr == nil {
		assert.Error(t, closeErr)
	} else {
		assert.Error(t, writeErr)
	}
}

func Test_Close(t *testing.T) {
	tgz := common.NewTarGzipWriter(&MockWriter{})
	require.NotNil(t, tgz)

	err := tgz.WriteHeader(testTarHeader)
	require.NoError(t, err)

	n, err := tgz.Write(testData)
	require.NoError(t, err)
	require.Equal(t, len(testData), n)

	err = tgz.Close()
	assert.NoError(t, err)

	err = tgz.Close()
	assert.NoError(t, err)

	_, err = tgz.Write(testData)
	assert.Error(t, err)

	err = tgz.WriteHeader(testTarHeader)
	assert.Error(t, err)
}
