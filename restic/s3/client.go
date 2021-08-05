package s3

import (
	"fmt"
	"io"
	"net/url"
	"strings"

	minio "github.com/minio/minio-go"
)

// Client wraps the minio s3 client
type Client struct {
	Endpoint        string
	AccessKeyID     string
	SecretAccessKey string
	minioClient     *minio.Client
	bucket          string
}

type UploadObject struct {
	ObjectStream io.Reader
	Name         string
}

// New returns a new Client
func New(endpoint, accessKeyID, secretAccessKey string) *Client {
	return &Client{
		Endpoint:        endpoint,
		AccessKeyID:     accessKeyID,
		SecretAccessKey: secretAccessKey,
	}
}

// Connect creates a minio client
func (c *Client) Connect() error {

	u, err := url.Parse(c.Endpoint)
	if err != nil {
		return fmt.Errorf("Error parsing S3 Endpoint URL: %w", err)
	}

	var ssl bool
	if u.Scheme == "https" {
		ssl = true
	} else if u.Scheme == "http" {
		ssl = false
	} else {
		return fmt.Errorf("endpoint '%v' has wrong scheme '%s' (should be 'http' or 'https')", c.Endpoint, u.Scheme)
	}

	c.bucket = strings.Replace(u.Path, "/", "", 1)
	c.Endpoint = u.Host
	mc, err := minio.New(c.Endpoint, c.AccessKeyID, c.SecretAccessKey, ssl)
	c.minioClient = mc

	if err == nil {
		err = c.createBucket()
	}

	return err
}

func (c *Client) createBucket() error {
	exists, err := c.minioClient.BucketExists(c.bucket)
	// Workaround for upstream bug -> australian s3 returns error on non existing bucket.
	if !exists && (err == nil || strings.Contains(err.Error(), "exist")) {
		return c.minioClient.MakeBucket(c.bucket, "")
	} else if err != nil {
		return err
	}
	return nil
}

// Upload uploads a io.Reader object to the configured endpoint
func (c *Client) Upload(object UploadObject) error {
	_, err := c.minioClient.PutObject(c.bucket, object.Name, object.ObjectStream, -1, minio.PutObjectOptions{})
	return err
}

// Get gets a file or returns an error.
func (c *Client) Get(filename string) (*minio.Object, error) {
	return c.minioClient.GetObject(c.bucket, filename, minio.GetObjectOptions{})
}

// Stat returns metainformation about an object in the repository.
func (c *Client) Stat(filename string) (minio.ObjectInfo, error) {
	return c.minioClient.StatObject(c.bucket, filename, minio.StatObjectOptions{})
}

// DeleteBucket deletes the main bucket where the client is connected to.
func (c *Client) DeleteBucket() error {
	return c.deleteBucketByName(c.bucket)
}

// DeleteBucketByName deletes the bucket with the specified name
func (c *Client) DeleteBucketByName(name string) error {
	return c.deleteBucketByName(name)
}

func (c *Client) deleteBucketByName(name string) error {

	objectsCh := make(chan string)

	// Send object names that are needed to be removed to objectsCh
	go func() {
		defer close(objectsCh)

		doneCh := make(chan struct{})

		// Indicate to our routine to exit cleanly upon return.
		defer close(doneCh)

		// List all objects from a bucket-name with a matching prefix.
		for object := range c.minioClient.ListObjects(name, "", true, doneCh) {
			objectsCh <- object.Key
		}
	}()

	// Call RemoveObjects API
	errorCh := c.minioClient.RemoveObjects(name, objectsCh)

	// Print errors received from RemoveObjects API
	for e := range errorCh {
		return fmt.Errorf("Failed to remove " + e.ObjectName + ", error: " + e.Err.Error())
	}

	return c.minioClient.RemoveBucket(name)
}

// ListObjects lists all objects in the bucket
func (c *Client) ListObjects() ([]minio.ObjectInfo, error) {
	doneCh := make(chan struct{})

	defer close(doneCh)

	tmpInfos := []minio.ObjectInfo{}

	isRecursive := true
	objectCh := c.minioClient.ListObjectsV2(c.bucket, "", isRecursive, doneCh)
	for object := range objectCh {
		if object.Err != nil {
			return nil, object.Err
		}
		tmpInfos = append(tmpInfos, object)
	}

	return tmpInfos, nil
}
