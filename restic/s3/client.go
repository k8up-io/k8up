package s3

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// Client wraps the minio s3 client
type Client struct {
	Endpoint        string
	AccessKeyID     string
	SecretAccessKey string
	minioClient     *minio.Client
	bucket          string
	cert            Cert
}

type Cert struct {
	CACert     string
	ClientCert string
	ClientKey  string
}

type UploadObject struct {
	ObjectStream io.Reader
	Name         string
}

// New returns a new Client
func New(endpoint, accessKeyID, secretAccessKey string, cert Cert) *Client {
	return &Client{
		Endpoint:        endpoint,
		AccessKeyID:     accessKeyID,
		SecretAccessKey: secretAccessKey,
		cert:            cert,
	}
}

// Connect creates a minio client
func (c *Client) Connect(ctx context.Context) error {
	u, err := url.Parse(c.Endpoint)
	if err != nil {
		return fmt.Errorf("error parsing S3 Endpoint URL: %w", err)
	}

	var ssl bool
	switch u.Scheme {
	case "https":
		ssl = true
	case "http":
		ssl = false
	default:
		return fmt.Errorf("endpoint '%v' has wrong scheme '%s' (should be 'http' or 'https')", c.Endpoint, u.Scheme)
	}

	var transportTlsConfig = &tls.Config{}
	if c.cert.CACert != "" {
		caCert, err := os.ReadFile(c.cert.CACert)
		if err != nil {
			return err
		}
		caCertPool := x509.NewCertPool()
		caCertPool.AppendCertsFromPEM(caCert)

		transportTlsConfig.RootCAs = caCertPool
	}
	if c.cert.ClientCert != "" && c.cert.ClientKey != "" {
		clientCert, err := tls.LoadX509KeyPair(c.cert.ClientCert, c.cert.ClientKey)
		if err != nil {
			return err
		}

		transportTlsConfig.Certificates = []tls.Certificate{clientCert}
	}

	var TransportRoundTripper http.RoundTripper = &http.Transport{
		TLSClientConfig: transportTlsConfig,
	}

	c.bucket = strings.Replace(u.Path, "/", "", 1)
	c.Endpoint = u.Host
	mc, err := minio.New(c.Endpoint, &minio.Options{
		Creds:     credentials.NewStaticV2(c.AccessKeyID, c.SecretAccessKey, ""),
		Secure:    ssl,
		Transport: TransportRoundTripper,
	})
	c.minioClient = mc

	if err == nil {
		err = c.createBucket(ctx)
	}

	return err
}

func (c *Client) createBucket(ctx context.Context) error {
	exists, err := c.minioClient.BucketExists(ctx, c.bucket)
	// Workaround for upstream bug -> australian s3 returns error on non existing bucket.
	if !exists && (err == nil || strings.Contains(err.Error(), "exist")) {
		return c.minioClient.MakeBucket(ctx, c.bucket, minio.MakeBucketOptions{})
	} else if err != nil {
		return err
	}
	return nil
}

// Upload uploads a io.Reader object to the configured endpoint
func (c *Client) Upload(ctx context.Context, object UploadObject) error {
	_, err := c.minioClient.PutObject(ctx, c.bucket, object.Name, object.ObjectStream, -1, minio.PutObjectOptions{})
	return err
}

// Get gets a file or returns an error.
func (c *Client) Get(ctx context.Context, filename string) (*minio.Object, error) {
	return c.minioClient.GetObject(ctx, c.bucket, filename, minio.GetObjectOptions{})
}

// Stat returns metainformation about an object in the repository.
func (c *Client) Stat(ctx context.Context, filename string) (minio.ObjectInfo, error) {
	return c.minioClient.StatObject(ctx, c.bucket, filename, minio.StatObjectOptions{})
}

// DeleteBucket deletes the main bucket where the client is connected to.
func (c *Client) DeleteBucket(ctx context.Context) error {
	return c.deleteBucketByName(ctx, c.bucket)
}

// DeleteBucketByName deletes the bucket with the specified name
func (c *Client) DeleteBucketByName(ctx context.Context, name string) error {
	return c.deleteBucketByName(ctx, name)
}

func (c *Client) deleteBucketByName(ctx context.Context, name string) error {
	// Send object names that are needed to be removed to objectsCh
	objectsCh := c.minioClient.ListObjects(ctx, name, minio.ListObjectsOptions{Recursive: true})

	// Call RemoveObjects API
	errorCh := c.minioClient.RemoveObjects(ctx, name, objectsCh, minio.RemoveObjectsOptions{})

	// Print errors received from RemoveObjects API
	for e := range errorCh {
		return fmt.Errorf("failed to remove %v ,error: %v", e.ObjectName, e.Err.Error())
	}

	return c.minioClient.RemoveBucket(ctx, name)
}

// ListObjects lists all objects in the bucket
func (c *Client) ListObjects(ctx context.Context) ([]minio.ObjectInfo, error) {
	doneCh := make(chan struct{})

	defer close(doneCh)

	tmpInfos := make([]minio.ObjectInfo, 0)
	objectCh := c.minioClient.ListObjects(ctx, c.bucket, minio.ListObjectsOptions{Recursive: true})
	for object := range objectCh {
		if object.Err != nil {
			return nil, object.Err
		}
		tmpInfos = append(tmpInfos, object)
	}

	return tmpInfos, nil
}
