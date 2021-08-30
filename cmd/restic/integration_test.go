// +build integration

package restic

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

	"github.com/vshn/k8up/restic/cli"
	"github.com/vshn/k8up/restic/s3"
	"github.com/vshn/k8up/restic/stats"
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

const testfileContent = "data\n"

type testEnvironment struct {
	s3Client  *s3.Client
	webhook   *webhookserver
	finishC   chan error
	t         *testing.T
	resticCli *cli.Restic
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
	statHandler := stats.NewHandler(os.Getenv(promURLEnv), os.Getenv(cli.Hostname), os.Getenv(webhookURLEnv), mainLogger)
	resticCli := cli.New(context.TODO(), mainLogger, statHandler)

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
	ctx := context.Background()
	repo := getS3Repo()
	s3client := s3.New(repo, os.Getenv("AWS_ACCESS_KEY_ID"), os.Getenv("AWS_SECRET_ACCESS_KEY"))

	err := s3client.Connect(ctx)
	require.NoErrorf(t, err, "Unable to connect to S3 repo '%s'", repo)
	t.Logf("Connected to S3 repo '%s'", repo)

	_ = s3client.DeleteBucket(ctx)
	t.Logf("Ensured that the bucket '%s' does not exist", repo)

	t.Cleanup(func() {
		_ = s3client.DeleteBucket(ctx)
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
	backupDirEnv, ok := os.LookupEnv(cli.BackupDirEnv)
	require.Truef(t, ok, "%s is not defined.", cli.BackupDirEnv)

	restoreDirEnv, ok := os.LookupEnv(cli.RestoreDirEnv)
	require.Truef(t, ok, "%s is not defined.", cli.RestoreDirEnv)

	dirs := []string{
		backupDirEnv,
		restoreDirEnv,
	}

	for _, dir := range dirs {
		_ = os.RemoveAll(dir)
		t.Logf("Ensured '%s' does not exist", dir)
	}
}

func createTestFiles(t *testing.T) {
	baseDir := os.Getenv(cli.BackupDirEnv)

	testDirs := []string{"PVC1", "PVC2"}
	for _, subDir := range testDirs {
		dir := filepath.Join(baseDir, subDir)
		file := filepath.Join(dir, "test.txt")

		err := os.MkdirAll(dir, os.ModePerm)
		require.NoError(t, err)

		err = ioutil.WriteFile(file, []byte(testfileContent), os.ModePerm)
		require.NoError(t, err)

		abs, _ := filepath.Abs(file)
		t.Logf("Created '%s'", abs)
	}
}

func resetFlags() {
	check = false
	prune = false
	restore = false
	restoreSnap = ""
	verifyRestore = false
	restoreType = ""
	restoreFilter = ""
	archive = false
}

func testBackup(t *testing.T) *testEnvironment {
	env := initTest(t)

	resticCli := env.resticCli
	err := run(context.Background(), resticCli, env.log)
	require.NoError(t, err)

	return env
}

func testCheckS3Restore(t *testing.T) {
	ctx := context.Background()
	s3c := s3.New(os.Getenv("RESTORE_S3ENDPOINT"), os.Getenv("RESTORE_ACCESSKEYID"), os.Getenv("RESTORE_SECRETACCESSKEY"))
	err := s3c.Connect(ctx)
	require.NoError(t, err)
	files, err := s3c.ListObjects(ctx)
	require.NoError(t, err)

	for _, file := range files {
		if strings.Contains(file.Key, "backup-test") {
			file, err := s3c.Get(ctx, file.Key)
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

	restore = true
	restoreType = "s3"

	err := run(context.Background(), env.resticCli, env.log)
	require.NoError(t, err)

	webhookData := cli.RestoreStats{}
	err = json.Unmarshal(env.webhook.jsonData, &webhookData)
	require.NoError(t, err)

	if webhookData.SnapshotID == "" {
		t.Errorf("No restore webhooks detected!")
	}

	testCheckS3Restore(t)
}

func TestBackup(t *testing.T) {
	env := testBackup(t)

	webhookData := cli.BackupStats{}
	err := json.Unmarshal(env.webhook.jsonData, &webhookData)
	require.NoError(t, err)

	assert.Len(t, webhookData.Snapshots, 2, "Not exactly two snapshot in the repository.")
}

func TestRestoreDisk(t *testing.T) {
	env := testBackup(t)

	restore = true
	restoreType = "folder"

	_ = os.Setenv("TRIM_RESTOREPATH", "false")

	err := run(context.Background(), env.resticCli, env.log)
	require.NoError(t, err)

	restoredir := os.Getenv(cli.RestoreDirEnv)
	backupdir := os.Getenv(cli.BackupDirEnv)
	restoreFilePath := filepath.Join(restoredir, backupdir, "PVC2/test.txt")
	contents, err := ioutil.ReadFile(restoreFilePath)
	require.NoError(t, err)

	assert.Equalf(t, testfileContent, string(contents), "restored content of '%s' is not as expected", restoreFilePath)
}

func TestArchive(t *testing.T) {
	env := testBackup(t)

	archive = true
	restoreType = "s3"

	err := run(context.Background(), env.resticCli, env.log)
	require.NoError(t, err)

	testCheckS3Restore(t)
}
