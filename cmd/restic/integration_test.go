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
	"time"

	"github.com/go-logr/glogr"
	"github.com/go-logr/logr"
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
	s.Server.Shutdown(ctx)
	time.Sleep(1 * time.Second)
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

func assertOK(t *testing.T, err error) {
	if err != nil {
		t.Error(err)
	}
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
		if err := srv.ListenAndServe(); err != http.ErrServerClosed {
			assertOK(t, err)
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

	webhook := &webhookserver{}
	webhook.runWebServer(t)
	s3client := s3.New(getS3Repo(), os.Getenv("AWS_ACCESS_KEY_ID"), os.Getenv("AWS_SECRET_ACCESS_KEY"))
	s3client.Connect()
	s3client.DeleteBucket()
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

	assertOK(t, run(env.resticCli, env.log))

	return env
}

func testResetEnvVars(vars []string) {
	for _, envVar := range vars {
		split := strings.Split(envVar, "=")
		os.Setenv(split[0], split[1])
	}
}

func testCheckS3Restore(t *testing.T) {
	s3c := s3.New(os.Getenv("RESTORE_S3ENDPOINT"), os.Getenv("RESTORE_ACCESSKEYID"), os.Getenv("RESTORE_SECRETACCESSKEY"))
	s3c.Connect()
	files, err := s3c.ListObjects()
	assertOK(t, err)

	for _, file := range files {
		if strings.Contains(file.Key, "backup-test") {
			file, err := s3c.Get(file.Key)
			assertOK(t, err)
			gzpReader, err := gzip.NewReader(file)
			assertOK(t, err)
			tarReader := tar.NewReader(gzpReader)

			var contents bytes.Buffer
			for {
				header, err := tarReader.Next()

				if err == io.EOF {
					break
				} else if err != nil {
					assertOK(t, err)
					break
				}

				if header.Typeflag == tar.TypeReg {
					io.Copy(&contents, tarReader)
				}
			}

			assertOK(t, err)
			if strings.TrimSpace(contents.String()) != "data" {
				t.Error("restored contents is not \"data\" but: ", contents.String())
			}
			break
		}
	}
}

func TestRestore(t *testing.T) {
	env := testBackup(t)

	restoreBool := true
	restore = &restoreBool
	rstType := "s3"
	restoreType = &rstType

	assertOK(t, run(env.resticCli, env.log))

	env.webhook.srv.Shutdown(context.TODO())

	testCheckS3Restore(t)

}

func TestBackup(t *testing.T) {
	env := testBackup(t)

	webhookData := restic.BackupStats{}
	assertOK(t, json.Unmarshal(env.webhook.jsonData, &webhookData))

	if len(webhookData.Snapshots) != 2 {
		t.Errorf("Not exactly two snapshot in the repository, but: %v", len(webhookData.Snapshots))
	}

	env.webhook.srv.Shutdown(context.TODO())

}

func TestRestoreDisk(t *testing.T) {
	env := testBackup(t)

	restoreBool := true
	restore = &restoreBool
	rstType := "folder"
	restoreType = &rstType

	os.Setenv("TRIM_RESTOREPATH", "false")

	assertOK(t, run(env.resticCli, env.log))

	env.webhook.srv.Shutdown(context.TODO())

	contents, err := ioutil.ReadFile(filepath.Join(os.Getenv(restic.RestoreDirEnv), "testdata/PVC2/test.txt"))
	assertOK(t, err)

	if strings.TrimSpace(string(contents)) != "data" {
		t.Error("restored contents is not \"data\" but: ", string(contents))
	}
}

func TestInitRepoFail(t *testing.T) {
	oldEnvVars := os.Environ()

	os.Setenv("RESTIC_REPOSITORY", "s3:http://localhost:1337/blah")
	env := initTest(t)
	defer testResetEnvVars(oldEnvVars)

	err := run(env.resticCli, env.log)

	if err == nil || !strings.Contains(err.Error(), "exit status 1") {
		t.Errorf("command did not fail with expected error, received error was: %v", err)
	}

	env.webhook.srv.Shutdown(context.TODO())

}

func TestArchive(t *testing.T) {
	env := testBackup(t)

	archiveBool := true
	archive = &archiveBool
	restoreTypeVar := "s3"
	restoreType = &restoreTypeVar

	assertOK(t, run(env.resticCli, env.log))

	env.webhook.srv.Shutdown(context.TODO())

	testCheckS3Restore(t)

}
