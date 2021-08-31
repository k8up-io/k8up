package common

import (
	"archive/tar"
	"compress/gzip"
	"io"
)

// TarGzipWriter consists of a pair of writers, namely a tar.Writer and gzip.Writer.
// They are combined such that a valid `tar.gz` stream is created.

// TarGzipWriter is a valid io.WriteCloser.
// It implements WriteHeader from tar.Writer to create separate files within the tar archive.
type TarGzipWriter struct {
	tarWriter  *tar.Writer
	gzipWriter *gzip.Writer
}

// NewTarGzipWriter creates a new TarGzipWriter.
func NewTarGzipWriter(w io.Writer) *TarGzipWriter {
	gzipWriter := gzip.NewWriter(w)
	tarWriter := tar.NewWriter(gzipWriter)

	return &TarGzipWriter{
		tarWriter:  tarWriter,
		gzipWriter: gzipWriter,
	}
}

// WriteHeader starts a new file in the tar archive; see tar.Writer.
func (t *TarGzipWriter) WriteHeader(hdr *tar.Header) error {
	return t.tarWriter.WriteHeader(hdr)
}

// Write adds content to the current file in the tar gzip archive; see tar.Writer.
func (t *TarGzipWriter) Write(p []byte) (int, error) {
	return t.tarWriter.Write(p)
}

// Close closes the inner tar.Writer and then subsequently the outer gzip.Writer.
//
// It returns the error of either call after both writers have been closed.
// If both calls to each writer.Close() error, then the error of closing the gzip.Writer is returned.
//
// The downstream writer is left as it is, i.e. it must be closed independently.
func (t *TarGzipWriter) Close() error {
	tarErr := t.tarWriter.Close()
	gzipErr := t.gzipWriter.Close()
	if gzipErr != nil {
		return gzipErr
	}
	return tarErr
}
