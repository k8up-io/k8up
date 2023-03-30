package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"

	v1 "github.com/k8up-io/k8up/v2/api/v1"
	"github.com/k8up-io/k8up/v2/cli/restore"
	"github.com/k8up-io/k8up/v2/cmd"
	"github.com/urfave/cli/v2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

var (
	Command = &cli.Command{
		Name:        "cli",
		Description: "CLI commands that can be executed everywhere, currently just restore is supported",
		Subcommands: []*cli.Command{
			{
				Name:   "restore",
				Action: RunRestore,
				Flags: []cli.Flag{
					&cli.StringFlag{
						Destination: &restore.Cfg.Snapshot,
						Required:    false,
						Name:        "snapshot",
						Value:       "latest",
						Usage:       "Optional ; ID of the snapshot `kubectl get snapshots`, if left empty 'latest' will be used, set it via cli or via env: ",
						EnvVars: []string{
							"SNAPSHOT",
						},
					},
					&cli.StringFlag{
						Destination: &restore.Cfg.SecretRef,
						Required:    true,
						Name:        "secretRef",
						Usage:       "Required ; Set secret name from which You want to take S3 credentials, via cli or via env: ",
						EnvVars: []string{
							"SECRET_REF",
						},
					},
					&cli.StringFlag{
						Destination: &restore.Cfg.S3Endpoint,
						Required:    true,
						Name:        "s3endpoint",
						Usage:       "Required ; Set s3endpoint from which backup will be taken, via cli or via env: ",
						EnvVars: []string{
							"S3ENDPOINT",
						},
					},
					&cli.StringFlag{
						Destination: &restore.Cfg.S3Bucket,
						Required:    true,
						Name:        "s3bucket",
						Usage:       "Required ; Set s3bucket from which backup will be taken, via cli or via env: ",
						EnvVars: []string{
							"S3BUCKET",
						},
					},
					&cli.StringFlag{
						Destination: &restore.Cfg.S3SecretRef,
						Required:    true,
						Name:        "s3secretRef",
						Usage:       "Required ; Set secret name, where S3 username & password are stored from which backup will be taken, via cli or via env: ",
						EnvVars: []string{
							"S3SECRETREF",
						},
					},
					&cli.StringFlag{
						Destination: &restore.Cfg.RestoreMethod,
						Required:    true,
						Name:        "restoreMethod",
						Value:       "pvc",
						Usage:       "Required ; Set restore method [ pvc|s3 ], via cli or via env: ",
						Action: func(ctx *cli.Context, s string) error {
							if s != "pvc" && s != "s3" {
								return fmt.Errorf("--restoreMethod must be set to either 'pvc' or 's3'")
							}
							return nil
						},
						EnvVars: []string{
							"RESTOREMETHOD",
						},
					},
					&cli.StringFlag{
						Destination: &restore.Cfg.SecretRefKey,
						Required:    false,
						Name:        "secretRefKey",
						Value:       "password",
						Usage:       "Optional ; Set key name, where restic password is stored, via cli or via env: ",
						EnvVars: []string{
							"SECRETREFKEY",
						},
					},
					&cli.StringFlag{
						Destination: &restore.Cfg.ClaimName,
						Required:    false,
						Name:        "claimName",
						Usage:       "Required ; Set claimName field, via cli or via env: ",
						EnvVars: []string{
							"CLAIMNAME",
						},
					},
					&cli.StringFlag{
						Destination: &restore.Cfg.S3SecretRefUsernameKey,
						Required:    false,
						Value:       "username",
						Name:        "S3SecretRefUsernameKey",
						Usage:       "Optional ; Set S3SecretRefUsernameKey, key inside secret, under which S3 username is stored, via cli or via env: ",
						EnvVars: []string{
							"S3SECRETREFUSERNAMEKEY",
						},
					},
					&cli.StringFlag{
						Destination: &restore.Cfg.S3SecretRefPasswordKey,
						Required:    false,
						Value:       "password",
						Name:        "S3SecretRefPasswordKey",
						Usage:       "Optional ; Set S3SecretRefPasswordKey, key inside secret, under which Restic repo password is stored, via cli or via env: ",
						EnvVars: []string{
							"S3SECRETREFPASSWORDKEY",
						},
					},
					&cli.StringFlag{
						Destination: &restore.Cfg.RestoreName,
						Required:    false,
						Name:        "restoreName",
						Usage:       "Optional ; Set restoreName - metadata.Name field, if empty, k8up will generate name, via cli or via env: ",
						EnvVars: []string{
							"RESTORENAME",
						},
					},
					&cli.Int64Flag{
						Destination: &restore.Cfg.RunAsUser,
						Required:    false,
						Name:        "runAsUser",
						Usage:       "Optional ; Set user UID, via cli or via env: ",
						EnvVars: []string{
							"RUNASUSER",
						},
					},
					&cli.StringFlag{
						Destination: &restore.Cfg.RestoreToS3Endpoint,
						Required:    false,
						Name:        "restoreToS3Endpoint",
						Usage:       "Optional ; Set restore endpoint, only when using s3 restore method, via cli or via env: ",
						EnvVars: []string{
							"RESTORETOS3ENDPOINT",
						},
					},
					&cli.StringFlag{
						Destination: &restore.Cfg.RestoreToS3Bucket,
						Required:    false,
						Name:        "restoreToS3Bucket",
						Usage:       "Optional ; Set restore bucket, only when using s3 restore method, via cli or via env: ",
						EnvVars: []string{
							"RESTORETOS3BUCKET",
						},
					},
					&cli.StringFlag{
						Destination: &restore.Cfg.RestoreToS3Secret,
						Required:    false,
						Name:        "restoreToS3Secret",
						Usage:       "Optional ; Set restore Secret, only when using s3 restore method, expecting secret name containing key value pair with 'username' and 'password' keys, via cli or via env: ",
						EnvVars: []string{
							"RESTORETOS3SECRET",
						},
					},
					&cli.StringFlag{
						Destination: &restore.Cfg.RestoreToS3SecretUsernameKey,
						Required:    false,
						Value:       "username",
						Name:        "RestoreToS3SecretUsernameKey",
						Usage:       "Optional ; Set RestoreToS3SecretUsernameKey, key inside secret, under which S3 username is stored, via cli or via env: ",
						EnvVars: []string{
							"RESTORETOS3SECRETUSERNAMEKEY",
						},
					},
					&cli.StringFlag{
						Destination: &restore.Cfg.RestoreToS3SecretPasswordKey,
						Required:    false,
						Value:       "password",
						Name:        "RestoreToS3SecretPasswordKey",
						Usage:       "Optional ; Set RestoreToS3SecretPasswordKey, key inside secret, under which Restic repo password is stored, via cli or via env: ",
						EnvVars: []string{
							"RESTORETOS3SECRETPASSWORDKEY",
						},
					},
					&cli.StringFlag{
						Destination: &restore.Cfg.Namespace,
						Required:    false,
						Value:       "default",
						Aliases:     []string{"n"},
						Name:        "namespace",
						Usage:       "Optional ; Set namespace in which You want to execute restore, via cli or via env: ",
						EnvVars: []string{
							"NAMESPACE",
						},
					},
					&cli.StringFlag{
						Destination: &restore.Cfg.Kubeconfig,
						Required:    false,
						Value:       "~/.kube/config",
						Name:        "kubeconfig",
						Usage:       "Optional ; Set kubeconfig to connect to cluster, via cli or via env:",
						EnvVars: []string{
							"KUBECONFIG",
						},
					},
				},
			},
		},
	}
)

func RunRestore(ctx *cli.Context) error {
	var restoreName, snapshot string
	var s3, pvc v1.RestoreMethod
	logger := cmd.AppLogger(ctx).WithName("cli-restore")

	// avoid name crashes + generate name if naming is not important for user
	if restore.Cfg.RestoreName == "" {
		restoreName = "cli-restore-" + restore.RandomStringGenerator(5)
		log.Println("Creating restore with name: ", restoreName, " in napespace: ", restore.Cfg.Namespace)
	} else {
		restoreName = restore.Cfg.RestoreName
	}

	kconfig, err := clientcmd.BuildConfigFromFlags("", restore.Cfg.Kubeconfig)
	if err != nil {
		logger.Error(err, "Failed to create KubeConfig")
		return err
	}
	logger.Info("Found Kubeconfig", "KUBECONFIG", restore.Cfg.Kubeconfig)

	client, err := kubernetes.NewForConfig(kconfig)
	if err != nil {
		logger.Error(err, "Failed to create Kubernetes Client")
		return err
	}

	crd, err := client.RESTClient().Get().Namespace("default").AbsPath("/apis/k8up.io/v1/").Resource("snapshots").DoRaw(context.TODO())
	if err != nil {
		logger.Error(err, "Failed to query Kubernetes api for snapshots, is it correct cluster and namespace? Is k8up installed correctly?")
		return err
	}
	if restore.Cfg.Snapshot != "latest" {

		snpList := &v1.SnapshotList{}

		err = json.Unmarshal(crd, &snpList)
		if err != nil {
			logger.Error(err, "Failed to unmarshal Snapshots")
			return err
		}
		// kubectl get snapshots returns 8 character values, while snapshots itself are much longer
		// so it's simple enough to check prefix
		for _, snap := range snpList.Items {
			if strings.HasPrefix((*snap.Spec.ID), restore.Cfg.Snapshot) {
				snapshot = (*snap.Spec.ID)
				break
			}
		}
		// nothing to do if there are no snapshots
		if len(snapshot) == 0 {
			logger.Error(err, "Snapshot ID wasn't found")
			return err
		}
		logger.V(1).Info("Found correct snapshot", "SNAPSHOT", snapshot)
	}

	s3 = v1.RestoreMethod{
		S3: &v1.S3Spec{
			Endpoint: restore.Cfg.RestoreToS3Endpoint,
			Bucket:   restore.Cfg.RestoreToS3Bucket,
			AccessKeyIDSecretRef: &corev1.SecretKeySelector{
				Key: restore.Cfg.RestoreToS3SecretUsernameKey,
				LocalObjectReference: corev1.LocalObjectReference{
					Name: restore.Cfg.RestoreToS3Secret,
				},
			},
			SecretAccessKeySecretRef: &corev1.SecretKeySelector{
				Key: restore.Cfg.RestoreToS3SecretPasswordKey,
				LocalObjectReference: corev1.LocalObjectReference{
					Name: restore.Cfg.RestoreToS3Secret,
				},
			},
		},
	}

	pvc = v1.RestoreMethod{
		Folder: &v1.FolderRestore{
			PersistentVolumeClaimVolumeSource: &corev1.PersistentVolumeClaimVolumeSource{
				ClaimName: restore.Cfg.ClaimName,
			},
		},
	}

	restoreObject := v1.Restore{
		ObjectMeta: metav1.ObjectMeta{
			Name:      restoreName,
			Namespace: restore.Cfg.Namespace,
		},
		TypeMeta: metav1.TypeMeta{
			Kind:       "Restore",
			APIVersion: "k8up.io/v1",
		},
		Spec: v1.RestoreSpec{
			//Snapshot: snapshot,
			RunnableSpec: v1.RunnableSpec{
				Backend: &v1.Backend{
					RepoPasswordSecretRef: &corev1.SecretKeySelector{
						Key: restore.Cfg.SecretRefKey,
						LocalObjectReference: corev1.LocalObjectReference{
							Name: restore.Cfg.SecretRef,
						},
					},
					S3: &v1.S3Spec{
						Endpoint: restore.Cfg.S3Endpoint,
						Bucket:   restore.Cfg.S3Bucket,
						AccessKeyIDSecretRef: &corev1.SecretKeySelector{
							Key: restore.Cfg.S3SecretRefUsernameKey,
							LocalObjectReference: corev1.LocalObjectReference{
								Name: restore.Cfg.S3SecretRef,
							},
						},
						SecretAccessKeySecretRef: &corev1.SecretKeySelector{
							Key: restore.Cfg.S3SecretRefPasswordKey,
							LocalObjectReference: corev1.LocalObjectReference{
								Name: restore.Cfg.S3SecretRef,
							},
						},
					},
				},
			},
		},
	}
	if ctx.IsSet("runAsUser") {
		restoreObject.Spec.PodSecurityContext = &corev1.PodSecurityContext{
			RunAsUser: &restore.Cfg.RunAsUser,
		}
	}

	if restore.Cfg.RestoreMethod == "s3" {
		restoreObject.Spec.RestoreMethod = &s3
	} else {
		restoreObject.Spec.RestoreMethod = &pvc
	}

	yamled, err := json.Marshal(restoreObject)
	if err != nil {
		logger.Error(err, "Failed to marshal restoreObject")
		return err
	}
	post := client.RESTClient().Post().Namespace("default").AbsPath("/apis/k8up.io/v1/").Resource("restores").Body(yamled).Do(context.TODO())

	status, err := post.Raw()
	if err != nil {
		// lines below looks very weird, but it's necessary, as actual reason why request failed is hidden in status.Message variable
		var out1 metav1.Status
		err := json.Unmarshal(status, &out1)
		if err != nil {
			logger.Error(err, "Failed to unmarshal Status object")
		}
		logger.Error(err, out1.Message)
		return err
	}
	logger.Info(fmt.Sprintf("Backup created successfully, You can find it running:\tkubectl -n %s get restores.k8up.io %s", restore.Cfg.Namespace, restoreName))
	logger.Info(fmt.Sprintf("To access logs please run:\tkubectl -n %s logs jobs/restore-%s", restore.Cfg.Namespace, restoreName))
	return nil
}
