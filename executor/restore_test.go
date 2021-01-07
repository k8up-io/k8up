package executor

import (
	"context"
	"strings"

	k8upv1alpha1 "github.com/vshn/k8up/api/v1alpha1"
	"github.com/vshn/k8up/job"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	gtypes "github.com/onsi/gomega/types"

	batchv1 "k8s.io/api/batch/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
)

// These tests use Ginkgo (BDD-style Go testing framework). Refer to
// http://onsi.github.io/ginkgo/ to learn more about Ginkgo.

var (
	restCfg   *rest.Config
	k8sClient client.Client
	testEnv   *envtest.Environment
	scheme    = runtime.NewScheme()
)

var _ = BeforeSuite(func(done Done) {
	err := k8upv1alpha1.AddToScheme(scheme)
	Expect(err).NotTo(HaveOccurred())

	err = batchv1.AddToScheme(scheme)
	Expect(err).NotTo(HaveOccurred())

	close(done)
})

var _ = Describe("Restore", func() {
	type buildRestoreObjectCase struct {
		Restore     *k8upv1alpha1.Restore
		Config      *job.Config
		ExpectedJob *batchv1.Job
		JobMatcher  gtypes.GomegaMatcher
	}
	DescribeTable("buildRestoreObject", func(c buildRestoreObjectCase) {
		e := NewRestoreExecutor(*c.Config)
		j, err := e.buildRestoreObject(c.Restore)

		Expect(err).To(BeNil())
		Expect(j).NotTo(BeNil())
		Expect(j).To(PointTo(c.JobMatcher))
	},
		Entry("builds a job object for S3 restore", buildRestoreObjectCase{
			Restore: &k8upv1alpha1.Restore{
				Spec: k8upv1alpha1.RestoreSpec{
					RestoreMethod: &k8upv1alpha1.RestoreMethod{
						S3: &k8upv1alpha1.S3Spec{
							Endpoint: "http://localhost:9000",
							Bucket:   "test",
							AccessKeyIDSecretRef: &v1.SecretKeySelector{
								Key: "accessKey",
							},
							SecretAccessKeySecretRef: &v1.SecretKeySelector{
								Key: "secretKey",
							},
						},
					},
					RunnableSpec: k8upv1alpha1.RunnableSpec{
						Backend: &k8upv1alpha1.Backend{
							S3: &k8upv1alpha1.S3Spec{
								Endpoint: "http://localhost:9000",
								Bucket:   "test-backend",
								AccessKeyIDSecretRef: &v1.SecretKeySelector{
									Key: "accessKey-backend",
								},
								SecretAccessKeySecretRef: &v1.SecretKeySelector{
									Key: "secretKey-backend",
								},
							},
						},
					},
				},
			},
			Config: newConfig(),
			JobMatcher: jobMatcher("s3", nil, Elements{
				"RESTIC_REPOSITORY": MatchAllFields(Fields{
					"Name":      Equal("RESTIC_REPOSITORY"),
					"Value":     Equal("s3:http://localhost:9000/test-backend"),
					"ValueFrom": BeNil(),
				}),
				"RESTORE_S3ENDPOINT": MatchAllFields(Fields{
					"Name":      Equal("RESTORE_S3ENDPOINT"),
					"Value":     Equal("http://localhost:9000/test"),
					"ValueFrom": BeNil(),
				}),
				"RESTORE_ACCESSKEYID": MatchAllFields(Fields{
					"Name":  Equal("RESTORE_ACCESSKEYID"),
					"Value": Equal(""),
					"ValueFrom": PointTo(MatchFields(IgnoreExtras, Fields{
						"SecretKeyRef": PointTo(MatchFields(IgnoreExtras, Fields{
							"Key": Equal("accessKey"),
						})),
					})),
				}),
				"RESTORE_SECRETACCESSKEY": MatchAllFields(Fields{
					"Name":  Equal("RESTORE_SECRETACCESSKEY"),
					"Value": Equal(""),
					"ValueFrom": PointTo(MatchFields(IgnoreExtras, Fields{
						"SecretKeyRef": PointTo(MatchFields(IgnoreExtras, Fields{
							"Key": Equal("secretKey"),
						})),
					})),
				}),
				"AWS_ACCESS_KEY_ID": MatchAllFields(Fields{
					"Name":  Equal("AWS_ACCESS_KEY_ID"),
					"Value": Equal(""),
					"ValueFrom": PointTo(MatchFields(IgnoreExtras, Fields{
						"SecretKeyRef": PointTo(MatchFields(IgnoreExtras, Fields{
							"Key": Equal("accessKey-backend"),
						})),
					})),
				}),
				"AWS_SECRET_ACCESS_KEY": MatchAllFields(Fields{
					"Name":  Equal("AWS_SECRET_ACCESS_KEY"),
					"Value": Equal(""),
					"ValueFrom": PointTo(MatchFields(IgnoreExtras, Fields{
						"SecretKeyRef": PointTo(MatchFields(IgnoreExtras, Fields{
							"Key": Equal("secretKey-backend"),
						})),
					})),
				}),
			}, Elements{}, Elements{}),
		}),
		Entry("builds a job object for folder restore", buildRestoreObjectCase{
			Restore: &k8upv1alpha1.Restore{
				Spec: k8upv1alpha1.RestoreSpec{
					RestoreMethod: &k8upv1alpha1.RestoreMethod{
						Folder: &k8upv1alpha1.FolderRestore{
							PersistentVolumeClaimVolumeSource: &v1.PersistentVolumeClaimVolumeSource{
								ClaimName: "test",
								ReadOnly:  false,
							},
						},
					},
				},
			},
			Config: newConfig(),
			JobMatcher: jobMatcher("folder", nil, Elements{
				"AWS_ACCESS_KEY_ID": MatchAllFields(Fields{
					"Name":      Equal("AWS_ACCESS_KEY_ID"),
					"Value":     Equal(""),
					"ValueFrom": BeNil(),
				}),
				"AWS_SECRET_ACCESS_KEY": MatchAllFields(Fields{
					"Name":      Equal("AWS_SECRET_ACCESS_KEY"),
					"Value":     Equal(""),
					"ValueFrom": BeNil(),
				}),
				"RESTORE_DIR": MatchAllFields(Fields{
					"Name":      Equal("RESTORE_DIR"),
					"Value":     Equal("/restore"),
					"ValueFrom": BeNil(),
				}),
			}, Elements{
				"test": MatchFields(IgnoreExtras, Fields{
					"Name": Equal("test"),
					"VolumeSource": MatchFields(IgnoreExtras, Fields{
						"PersistentVolumeClaim": PointTo(MatchFields(IgnoreExtras, Fields{
							"ClaimName": Equal("test"),
							"ReadOnly":  Equal(false),
						})),
					}),
				}),
			}, Elements{
				"test": MatchFields(IgnoreExtras, Fields{
					"Name":      Equal("test"),
					"MountPath": Equal("/restore"),
				}),
			}),
		}),
		Entry("builds a job object with tags, filters and snapshot", buildRestoreObjectCase{
			Restore: &k8upv1alpha1.Restore{
				Spec: k8upv1alpha1.RestoreSpec{
					RestoreMethod: &k8upv1alpha1.RestoreMethod{
						Folder: &k8upv1alpha1.FolderRestore{
							PersistentVolumeClaimVolumeSource: &v1.PersistentVolumeClaimVolumeSource{
								ClaimName: "test",
								ReadOnly:  false,
							},
						},
					},
					Tags:          []string{"testtag", "another"},
					RestoreFilter: "testfilter",
					Snapshot:      "testsnapshot",
				},
			},
			Config: newConfig(),
			JobMatcher: jobMatcher("folder", []string{"--tag", "testtag", "--tag", "another", "-restoreFilter", "testfilter", "-restoreSnap", "testsnapshot"}, Elements{
				"AWS_ACCESS_KEY_ID": MatchAllFields(Fields{
					"Name":      Equal("AWS_ACCESS_KEY_ID"),
					"Value":     Equal(""),
					"ValueFrom": BeNil(),
				}),
				"AWS_SECRET_ACCESS_KEY": MatchAllFields(Fields{
					"Name":      Equal("AWS_SECRET_ACCESS_KEY"),
					"Value":     Equal(""),
					"ValueFrom": BeNil(),
				}),
				"RESTORE_DIR": MatchAllFields(Fields{
					"Name":      Equal("RESTORE_DIR"),
					"Value":     Equal("/restore"),
					"ValueFrom": BeNil(),
				}),
			}, Elements{
				"test": MatchFields(IgnoreExtras, Fields{
					"Name": Equal("test"),
					"VolumeSource": MatchFields(IgnoreExtras, Fields{
						"PersistentVolumeClaim": PointTo(MatchFields(IgnoreExtras, Fields{
							"ClaimName": Equal("test"),
							"ReadOnly":  Equal(false),
						})),
					}),
				}),
			}, Elements{
				"test": MatchFields(IgnoreExtras, Fields{
					"Name":      Equal("test"),
					"MountPath": Equal("/restore"),
				}),
			}),
		}),
	)
})

// gstruct `Identifier` functions (see: https://onsi.github.io/gomega/#gstruct-testing-complex-data-types)
func containerID(el interface{}) string {
	c := el.(v1.Container)
	return c.Image + strings.Join(c.Args, ",")
}
func envID(el interface{}) string {
	e := el.(v1.EnvVar)
	return e.Name
}
func volumeID(el interface{}) string {
	v := el.(v1.Volume)
	return v.Name
}
func volumeMountID(el interface{}) string {
	v := el.(v1.VolumeMount)
	return v.Name
}

func jobMatcher(restoreType string, additionalArgs []string, env Elements, volumes Elements, volumeMounts Elements) gtypes.GomegaMatcher {
	additionalArgs = append([]string{"-restore"}, additionalArgs...)
	additionalArgs = append(additionalArgs, "-restoreType", restoreType)

	env["HOSTNAME"] = MatchFields(IgnoreExtras, Fields{
		"Name": Equal("HOSTNAME"),
	})
	env["STATS_URL"] = MatchFields(IgnoreExtras, Fields{
		"Name": Equal("STATS_URL"),
	})
	env["RESTIC_PASSWORD"] = MatchFields(IgnoreExtras, Fields{
		"Name": Equal("RESTIC_PASSWORD"),
	})
	if _, ok := env["RESTIC_REPOSITORY"]; !ok {
		env["RESTIC_REPOSITORY"] = MatchFields(IgnoreExtras, Fields{
			"Name":  Equal("RESTIC_REPOSITORY"),
			"Value": Equal("s3:/"),
		})
	}

	return MatchFields(IgnoreExtras, Fields{
		"ObjectMeta": MatchFields(IgnoreExtras, Fields{
			"Labels": MatchAllKeys(Keys{
				"k8upjob":           Equal("true"),
				"k8upjob/exclusive": Equal("false"),
			}),
		}),
		"Spec": MatchFields(IgnoreExtras, Fields{
			"Template": MatchFields(IgnoreExtras, Fields{
				"Spec": MatchFields(IgnoreExtras, Fields{
					"Volumes": MatchAllElements(volumeID, volumes),
					"Containers": MatchAllElements(containerID, Elements{
						"172.30.1.1:5000/myproject/restic" + strings.Join(additionalArgs, ","): MatchFields(IgnoreExtras, Fields{
							"Image":        Equal("172.30.1.1:5000/myproject/restic"),
							"Args":         ContainElements(additionalArgs),
							"Env":          MatchAllElements(envID, env),
							"VolumeMounts": MatchAllElements(volumeMountID, volumeMounts),
						}),
					}),
				}),
			}),
		}),
	})
}

func newConfig() *job.Config {
	cfg := job.NewConfig(context.TODO(), nil, nil, &k8upv1alpha1.Restore{}, scheme, "")
	return &cfg
}
