// +build integration

package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-logr/glogr"
	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vshn/wrestic/s3"
	"github.com/vshn/wrestic/stats"

	"github.com/vshn/wrestic/restic"
)

type webhookserver struct {
	jsonData []byte
	srv      *testServer
}

type testServer struct {
	http.Server
}

func (s *testServer) Shutdown(ctx context.Context) {
	_ = s.Server.Shutdown(ctx)
}

type testEnvironment struct {
	s3Client  *s3.Client
	webhook   *webhookserver
	finishC   chan error
	t         *testing.T
	resticCli *restic.Restic
	log       logr.Logger
	stats     *stats.Handler
}

func newTestErrorChannel() chan error {
	return make(chan error)
}

func (w *webhookserver) runWebServer(t *testing.T) {

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(wr http.ResponseWriter, r *http.Request) {
		w.jsonData, _ = ioutil.ReadAll(r.Body)
	})

	srv := &testServer{
		Server: http.Server{
			Addr:    ":8091",
			Handler: mux,
		},
	}

	go func() {
		err := srv.ListenAndServe()
		if err != http.ErrServerClosed {
			require.NoError(t, err)
		}
	}()

	w.srv = srv
}

func getS3Repo() string {
	resticString := os.Getenv("RESTIC_REPOSITORY")
	resticString = strings.ToLower(resticString)

	return strings.Replace(resticString, "s3:", "", -1)
}

func initTest(t *testing.T) *testEnvironment {
	mainLogger := glogr.New().WithName("wrestic")
	statHandler := stats.NewHandler(os.Getenv(promURLEnv), os.Getenv(restic.Hostname), os.Getenv(webhookURLEnv), mainLogger)
	resticCli := restic.New(context.TODO(), mainLogger, statHandler)

	cleanupDirs(t)
	createTestFiles(t)
	t.Cleanup(func() {
		cleanupDirs(t)
	})

	webhook := startWebhookWebserver(t)
	s3client := connectToS3Server(t)
	resetFlags()

	return &testEnvironment{
		finishC:   newTestErrorChannel(),
		webhook:   webhook,
		s3Client:  s3client,
		t:         t,
		resticCli: resticCli,
		log:       mainLogger,
		stats:     statHandler,
	}
}

func connectToS3Server(t *testing.T) *s3.Client {
	repo := getS3Repo()
	s3client := s3.New(repo, os.Getenv("AWS_ACCESS_KEY_ID"), os.Getenv("AWS_SECRET_ACCESS_KEY"))

	err := s3client.Connect()
	require.NoError(t, err)
	t.Logf("Connected to S3 repo '%s'", repo)

	_ = s3client.DeleteBucket()
	t.Logf("Ensured that the bucket '%s' does not exist", repo)

	t.Cleanup(func() {
		_ = s3client.DeleteBucket()
		t.Logf("Removing the bucket '%s'", repo)
	})
	return s3client
}

func startWebhookWebserver(t *testing.T) *webhookserver {
	webhook := &webhookserver{}
	webhook.runWebServer(t)
	t.Logf("Started webserver on '%s'", webhook.srv.Addr)
	t.Cleanup(func() {
		if webhook.srv == nil {
			t.Log("Webserver not running.")
			return
		}

		t.Logf("Stopping the webserver on '%s'", webhook.srv.Addr)
		webhook.srv.Shutdown(context.Background())
	})
	return webhook
}

func cleanupDirs(t *testing.T) {
	dirs := []string{
		os.Getenv(restic.BackupDirEnv),
		os.Getenv(restic.RestoreDirEnv),
	}

	for _, dir := range dirs {
		_ = os.RemoveAll(dir)
		t.Logf("Ensured '%s' does not exist", dir)
	}
}

func createTestFiles(t *testing.T) {
	baseDir := os.Getenv(restic.BackupDirEnv)

	testDirs := []string{"PVC1", "PVC2"}
	for _, subDir := range testDirs {
		dir := filepath.Join(baseDir, subDir)
		file := filepath.Join(dir, "test.txt")

		err := os.MkdirAll(dir, os.ModePerm)
		require.NoError(t, err)

		err = ioutil.WriteFile(file, []byte("data\n"), os.ModePerm)
		require.NoError(t, err)

		t.Logf("Created '%s'", file)
	}
}

func resetFlags() {
	empty := ""
	falseBool := false
	check = &falseBool
	prune = &falseBool
	restore = &falseBool
	restoreSnap = &empty
	verifyRestore = &falseBool
	restoreType = &empty
	restoreFilter = &empty
	archive = &falseBool
}

func testBackup(t *testing.T) *testEnvironment {
	env := initTest(t)

	cli := env.resticCli
	err := run(cli, env.log)
	require.NoError(t, err)

	return env
}

func testCheckS3Restore(t *testing.T) {
	s3c := s3.New(os.Getenv("RESTORE_S3ENDPOINT"), os.Getenv("RESTORE_ACCESSKEYID"), os.Getenv("RESTORE_SECRETACCESSKEY"))
	err := s3c.Connect()
	require.NoError(t, err)
	files, err := s3c.ListObjects()
	require.NoError(t, err)

	for _, file := range files {
		if strings.Contains(file.Key, "backup-test") {
			file, err := s3c.Get(file.Key)
			require.NoError(t, err)
			gzpReader, err := gzip.NewReader(file)
			require.NoError(t, err)
			tarReader := tar.NewReader(gzpReader)

			var contents bytes.Buffer
			for {
				header, err := tarReader.Next()

				if err == io.EOF {
					break
				} else if err != nil {
					require.NoError(t, err)
					break
				}

				if header.Typeflag == tar.TypeReg {
					_, err := io.Copy(&contents, tarReader)
					require.NoError(t, err)
				}
			}

			require.NoError(t, err)
			if strings.TrimSpace(contents.String()) != "data" {
				t.Error("restored contents is not \"data\" but: ", contents.String())
			}
			break
		}
	}
}

func TestRestore(t *testing.T) {
	env := testBackup(t)
	defer env.webhook.srv.Shutdown(context.TODO())

	restoreBool := true
	restore = &restoreBool
	rstType := "s3"
	restoreType = &rstType

	err := run(env.resticCli, env.log)
	require.NoError(t, err)

	webhookData := restic.RestoreStats{}
	err = json.Unmarshal(env.webhook.jsonData, &webhookData)
	require.NoError(t, err)

	if webhookData.SnapshotID == "" {
		t.Errorf("No restore webhooks detected!")
	}

	testCheckS3Restore(t)
}

func TestBackup(t *testing.T) {
	env := testBackup(t)

	webhookData := restic.BackupStats{}
	err := json.Unmarshal(env.webhook.jsonData, &webhookData)
	require.NoError(t, err)

	assert.Len(t, webhookData.Snapshots, 2, "Not exactly two snapshot in the repository.")
}

func TestRestoreDisk(t *testing.T) {
	env := testBackup(t)

	restoreBool := true
	restore = &restoreBool
	rstType := "folder"
	restoreType = &rstType

	_ = os.Setenv("TRIM_RESTOREPATH", "false")

	err := run(env.resticCli, env.log)
	require.NoError(t, err)

	restoredir := os.Getenv(restic.RestoreDirEnv)
	backupdir := os.Getenv(restic.BackupDirEnv)
	restoreFilePath := filepath.Join(restoredir, backupdir, "PVC2/test.txt")
	contents, err := ioutil.ReadFile(restoreFilePath)
	require.NoError(t, err)

	assert.Equalf(t, "data\n", string(contents), "restored content of '%s' is not as expected", restoreFilePath)
}

func TestArchive(t *testing.T) {
	env := testBackup(t)

	archiveBool := true
	archive = &archiveBool
	restoreTypeVar := "s3"
	restoreType = &restoreTypeVar

	err := run(env.resticCli, env.log)
	require.NoError(t, err)

	testCheckS3Restore(t)
}
