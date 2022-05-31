//go:build integration

package restic

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-logr/logr"
	"github.com/go-logr/zapr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"github.com/k8up-io/k8up/v2/restic/cfg"
	"github.com/k8up-io/k8up/v2/restic/cli"
	"github.com/k8up-io/k8up/v2/restic/s3"
	"github.com/k8up-io/k8up/v2/restic/stats"
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
	ctx       context.Context
}

func newTestErrorChannel() chan error {
	return make(chan error)
}

func (w *webhookserver) runWebServer(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(wr http.ResponseWriter, r *http.Request) {
		w.jsonData, _ = io.ReadAll(r.Body)
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
	ctx := context.Background()

	cfg.Config = &cfg.Configuration{
		Hostname:   os.Getenv("HOSTNAME"),
		PromURL:    os.Getenv("PROM_URL"),
		WebhookURL: os.Getenv("STATS_URL"),

		RestoreS3Endpoint:  os.Getenv("RESTORE_S3ENDPOINT"),
		RestoreS3AccessKey: os.Getenv("RESTORE_ACCESSKEYID"),
		RestoreS3SecretKey: os.Getenv("RESTORE_SECRETACCESSKEY"),

		ResticBin:  os.Getenv("RESTIC_BINARY"),
		BackupDir:  os.Getenv("BACKUP_DIR"),
		RestoreDir: os.Getenv("RESTORE_DIR"),
	}

	mainLogger := zapr.NewLogger(zaptest.NewLogger(t))
	statHandler := stats.NewHandler(cfg.Config.PromURL, cfg.Config.Hostname, cfg.Config.WebhookURL, mainLogger)
	resticCli := cli.New(ctx, mainLogger, statHandler)

	cleanupDirs(t)
	createTestFiles(t)
	t.Cleanup(func() {
		cleanupDirs(t)
	})

	webhook := startWebhookWebserver(t, ctx)
	s3client := connectToS3Server(t, ctx)

	return &testEnvironment{
		finishC:   newTestErrorChannel(),
		webhook:   webhook,
		s3Client:  s3client,
		t:         t,
		resticCli: resticCli,
		log:       mainLogger,
		stats:     statHandler,
		ctx:       ctx,
	}
}

func connectToS3Server(t *testing.T, ctx context.Context) *s3.Client {
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

func startWebhookWebserver(t *testing.T, ctx context.Context) *webhookserver {
	webhook := &webhookserver{}
	webhook.runWebServer(t)
	t.Logf("Started webserver on '%s'", webhook.srv.Addr)
	t.Cleanup(func() {
		if webhook.srv == nil {
			t.Log("Webserver not running.")
			return
		}

		t.Logf("Stopping the webserver on '%s'", webhook.srv.Addr)
		webhook.srv.Shutdown(ctx)
	})
	return webhook
}

func cleanupDirs(t *testing.T) {
	backupDirEnv, ok := os.LookupEnv(backupDirEnvKey)
	require.Truef(t, ok, "%s is not defined.", backupDirEnv)

	restoreDirEnv, ok := os.LookupEnv(restoreDirEnvKey)
	require.Truef(t, ok, "%s is not defined.", restoreDirEnv)

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
	baseDir := os.Getenv(backupDirEnvKey)

	testDirs := []string{"PVC1", "PVC2"}
	for _, subDir := range testDirs {
		dir := filepath.Join(baseDir, subDir)
		file := filepath.Join(dir, "test.txt")

		err := os.MkdirAll(dir, os.ModePerm)
		require.NoError(t, err)

		err = os.WriteFile(file, []byte(testfileContent), os.ModePerm)
		require.NoError(t, err)

		abs, _ := filepath.Abs(file)
		t.Logf("Created '%s'", abs)
	}
}

func testBackup(t *testing.T) *testEnvironment {
	env := initTest(t)

	resticCli := env.resticCli
	err := run(env.ctx, resticCli, env.log)
	require.NoError(t, err)

	return env
}

func testCheckS3Restore(t *testing.T, ctx context.Context) {
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
	defer env.webhook.srv.Shutdown(env.ctx)

	cfg.Config.DoRestore = true
	cfg.Config.RestoreType = cfg.RestoreTypeS3

	err := run(env.ctx, env.resticCli, env.log)
	require.NoError(t, err)

	webhookData := cli.RestoreStats{}
	err = json.Unmarshal(env.webhook.jsonData, &webhookData)
	require.NoError(t, err)

	if webhookData.SnapshotID == "" {
		t.Errorf("No restore webhooks detected!")
	}

	testCheckS3Restore(t, env.ctx)
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

	cfg.Config.DoRestore = true
	cfg.Config.RestoreType = cfg.RestoreTypeFolder

	_ = os.Setenv("TRIM_RESTOREPATH", "false")

	err := run(env.ctx, env.resticCli, env.log)
	require.NoError(t, err)

	restoredir := os.Getenv(restoreDirEnvKey)
	backupdir := os.Getenv(backupDirEnvKey)
	restoreFilePath := filepath.Join(restoredir, backupdir, "PVC2/test.txt")
	contents, err := os.ReadFile(restoreFilePath)
	require.NoError(t, err)

	assert.Equalf(t, testfileContent, string(contents), "restored content of '%s' is not as expected", restoreFilePath)
}

func TestArchive(t *testing.T) {
	env := testBackup(t)

	cfg.Config.DoArchive = true
	cfg.Config.RestoreType = cfg.RestoreTypeS3

	err := run(env.ctx, env.resticCli, env.log)
	require.NoError(t, err)

	testCheckS3Restore(t, env.ctx)
}
