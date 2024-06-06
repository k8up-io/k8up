package cli

import (
	"io"
	"os"
)

func generatePemFile(clientCert string, clientKey string, dest string) error {
	certIn, err := os.Open(clientCert)
	if err != nil {
		return err
	}
	defer certIn.Close()

	tlsIn, err := os.Open(clientKey)
	if err != nil {
		return err
	}
	defer tlsIn.Close()

	out, err := os.OpenFile(dest, os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, certIn)
	if err != nil {
		return err
	}

	_, err = io.Copy(out, tlsIn)
	if err != nil {
		return err
	}

	return nil
}
