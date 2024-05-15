package schedulecontroller

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	k8upv1 "github.com/k8up-io/k8up/v2/api/v1"
	"github.com/k8up-io/k8up/v2/operator/cfg"
	"github.com/k8up-io/k8up/v2/operator/job"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
)

const (
	nsName = "myns"
)

func TestScheduleHandler_mergeResourcesWithDefaults(t *testing.T) {
	tests := map[string]struct {
		globalCPUResourceLimit      string
		globalCPUResourceRequest    string
		globalMemoryResourceLimit   string
		globalMemoryResourceRequest string
		givenScheduleTemplate       corev1.ResourceRequirements
		givenResourceTemplate       corev1.ResourceRequirements
		expectedTemplate            corev1.ResourceRequirements
	}{
		"Given_NoGlobalDefaults_And_NoScheduleDefaults_When_NoSpec_Then_LeaveEmpty": {
			expectedTemplate: corev1.ResourceRequirements{},
		},
		"Given_NoGlobalDefaults_And_NoScheduleDefaults_When_Spec_Then_UseSpec": {
			givenResourceTemplate: corev1.ResourceRequirements{
				Requests: newCPUResourceList("50m"),
			},
			expectedTemplate: corev1.ResourceRequirements{
				Requests: newCPUResourceList("50m"),
			},
		},
		"Given_NoGlobalDefaults_And_ScheduleDefaults_When_NoSpec_Then_ApplyScheduleDefaults": {
			givenScheduleTemplate: corev1.ResourceRequirements{
				Limits: newCPUResourceList("200m"),
			},
			expectedTemplate: corev1.ResourceRequirements{
				Limits: newCPUResourceList("200m"),
			},
		},
		"Given_NoGlobalDefaults_And_ScheduleDefaults_When_Spec_Then_UseSpec": {
			givenScheduleTemplate: corev1.ResourceRequirements{
				Limits: newCPUResourceList("200m"),
			},
			givenResourceTemplate: corev1.ResourceRequirements{
				Limits: newCPUResourceList("50m"),
			},
			expectedTemplate: corev1.ResourceRequirements{
				Limits: newCPUResourceList("50m"),
			},
		},
		"Given_GlobalDefaults_And_NoScheduleDefaults_When_NoSpec_Then_UseGlobalDefaults": {
			globalMemoryResourceRequest: "10Mi",
			givenScheduleTemplate: corev1.ResourceRequirements{
				Limits: newCPUResourceList("200m"),
			},
			expectedTemplate: corev1.ResourceRequirements{
				Limits: newCPUResourceList("200m"),
				Requests: corev1.ResourceList{
					corev1.ResourceMemory: resource.MustParse("10Mi"),
				},
			},
		},
		"Given_GlobalDefaults_And_NoScheduleDefaults_When_Spec_Then_UseSpec": {
			globalMemoryResourceRequest: "10Mi",
			givenResourceTemplate: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceMemory: resource.MustParse("20Mi"),
				},
			},
			expectedTemplate: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceMemory: resource.MustParse("20Mi"),
				},
			},
		},
		"Given_GlobalDefaults_And_ScheduleDefaults_When_NoSpec_Then_UseSchedule": {
			globalCPUResourceLimit: "10m",
			givenScheduleTemplate: corev1.ResourceRequirements{
				Limits: newCPUResourceList("200m"),
			},
			expectedTemplate: corev1.ResourceRequirements{
				Limits: newCPUResourceList("200m"),
			},
		},
		"Given_GlobalDefaults_And_ScheduleDefaults_When_Spec_Then_UseSpec": {
			globalCPUResourceLimit: "10m",
			givenScheduleTemplate: corev1.ResourceRequirements{
				Limits: newCPUResourceList("100m"),
			},
			givenResourceTemplate: corev1.ResourceRequirements{
				Limits: newCPUResourceList("200m"),
			},
			expectedTemplate: corev1.ResourceRequirements{
				Limits: newCPUResourceList("200m"),
			},
		},
	}
	cfg.Config = &cfg.Configuration{}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			cfg.Config.GlobalCPUResourceLimit = tt.globalCPUResourceLimit
			cfg.Config.GlobalCPUResourceRequest = tt.globalCPUResourceRequest
			cfg.Config.GlobalMemoryResourceLimit = tt.globalMemoryResourceLimit
			cfg.Config.GlobalMemoryResourceRequest = tt.globalMemoryResourceRequest
			schedule := ScheduleHandler{schedule: &k8upv1.Schedule{Spec: k8upv1.ScheduleSpec{
				ResourceRequirementsTemplate: tt.givenScheduleTemplate,
			}}}
			res := &k8upv1.RunnableSpec{
				Resources: tt.givenResourceTemplate,
			}
			schedule.mergeResourcesWithDefaults(res)
			assert.Equal(t, tt.expectedTemplate, res.Resources)
		})
	}
}

func newCPUResourceList(amount string) corev1.ResourceList {
	return corev1.ResourceList{
		corev1.ResourceCPU: resource.MustParse(amount),
	}
}

func TestScheduleHandler_mergeBackendWithDefaults(t *testing.T) {
	tests := map[string]struct {
		globalS3Bucket       string
		givenScheduleBackend k8upv1.Backend
		givenResourceBackend k8upv1.Backend
		expectedBackend      k8upv1.Backend
	}{
		"Given_NoGlobalDefaults_And_NoScheduleDefaults_When_Spec_Then_UseSpec": {
			givenResourceBackend: newS3Backend("https://resource-url", "resource-bucket"),
			expectedBackend:      newS3Backend("https://resource-url", "resource-bucket"),
		},
		"Given_NoGlobalDefaults_And_ScheduleDefaults_When_NoSpec_Then_ApplyScheduleDefaults": {
			givenScheduleBackend: newS3Backend("https://schedule-url", "schedule-bucket"),
			expectedBackend:      newS3Backend("https://schedule-url", "schedule-bucket"),
		},
		"Given_NoGlobalDefaults_And_ScheduleDefaults_When_Spec_Then_UseSpec": {
			givenScheduleBackend: newS3Backend("https://schedule-url", "schedule-bucket"),
			givenResourceBackend: newS3Backend("https://resource-url", "resource-bucket"),
			expectedBackend:      newS3Backend("https://resource-url", "resource-bucket"),
		},
		"Given_GlobalDefaults_And_NoScheduleDefaults_When_NoSpec_Then_UseGlobalDefaults": {
			globalS3Bucket:       "global-bucket",
			givenScheduleBackend: newS3Backend("https://schedule-url", ""),
			expectedBackend:      newS3Backend("https://schedule-url", ""),
		},
		"Given_GlobalDefaults_And_NoScheduleDefaults_When_Spec_Then_UseSpec": {
			globalS3Bucket:       "global-bucket",
			givenResourceBackend: newS3Backend("https://resource-url", "resource-bucket"),
			expectedBackend:      newS3Backend("https://resource-url", "resource-bucket"),
		},
		"Given_GlobalDefaults_And_ScheduleDefaults_When_NoSpec_Then_UseSchedule": {
			globalS3Bucket:       "global-bucket",
			givenScheduleBackend: newS3Backend("https://schedule-url", "schedule-bucket"),
			expectedBackend:      newS3Backend("https://schedule-url", "schedule-bucket"),
		},
		"Given_GlobalDefaults_And_ScheduleDefaults_When_Spec_Then_UseSpec": {
			globalS3Bucket:       "global-bucket",
			givenScheduleBackend: newS3Backend("https://schedule-url", "schedule-bucket"),
			givenResourceBackend: newS3Backend("https://resource-url", "resource-bucket"),
			expectedBackend:      newS3Backend("https://resource-url", "resource-bucket"),
		},
	}
	cfg.Config = &cfg.Configuration{}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			cfg.Config.GlobalS3Bucket = tt.globalS3Bucket
			schedule := ScheduleHandler{schedule: &k8upv1.Schedule{Spec: k8upv1.ScheduleSpec{
				Backend: &tt.givenScheduleBackend,
			}}}
			res := &k8upv1.RunnableSpec{
				Backend: &tt.givenResourceBackend,
			}
			schedule.mergeBackendWithDefaults(res)
			assert.NotNil(t, res.Backend.S3)
			assert.Equal(t, *tt.expectedBackend.S3, *res.Backend.S3)
		})
	}
}

func TestScheduleHandler_mergePodSecurityContextWithDefaults(t *testing.T) {
	tests := map[string]struct {
		givenSchedulePodSecurityContext *corev1.PodSecurityContext
		givenResourcePodSecurityContext *corev1.PodSecurityContext
		expectedPodSecurityContext      *corev1.PodSecurityContext
	}{
		"GivenScheduleContextIsNil_WhenResourceContextIsGiven_ThenUseResourcePSC": {
			givenSchedulePodSecurityContext: nil,
			givenResourcePodSecurityContext: newBuilderPSC().runAsUser(0).runAsGroup(1).seLinuxOptions("s0:c123,c456").get(),
			expectedPodSecurityContext:      newBuilderPSC().runAsUser(0).runAsGroup(1).seLinuxOptions("s0:c123,c456").get(),
		},
		"GivenScheduleContextIsPopulated_WhenResourceContextIsGiven_ThenMergeWithResourcePSC": {
			givenSchedulePodSecurityContext: newBuilderPSC().runAsGroup(2).seLinuxOptions("s1:c123,c456").get(),
			givenResourcePodSecurityContext: newBuilderPSC().runAsUser(1).runAsGroup(1).get(),
			expectedPodSecurityContext:      newBuilderPSC().runAsUser(1).runAsGroup(1).seLinuxOptions("s1:c123,c456").get(),
		},
		"GivenScheduleContextIsPopulated_WhenResourceContextIsNil_ThenUseResourcePSC": {
			givenSchedulePodSecurityContext: newBuilderPSC().runAsGroup(1).seLinuxOptions("s1:c123,c456").get(),
			givenResourcePodSecurityContext: nil,
			expectedPodSecurityContext:      newBuilderPSC().runAsGroup(1).seLinuxOptions("s1:c123,c456").get(),
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			schedule := ScheduleHandler{schedule: &k8upv1.Schedule{Spec: k8upv1.ScheduleSpec{
				PodSecurityContext: tt.givenSchedulePodSecurityContext,
			}}}
			res := &k8upv1.RunnableSpec{
				PodSecurityContext: tt.givenResourcePodSecurityContext,
			}
			schedule.mergeSecurityContextWithDefaults(res)
			assert.Equal(t, *tt.expectedPodSecurityContext, *res.PodSecurityContext)
		})
	}
}

type mockScheduler struct {
}

func (m *mockScheduler) HasSchedule(_ string) bool {
	return true
}

func (m *mockScheduler) RemoveSchedule(_ context.Context, _ string) {}

// SetSchedule will just apply the object to the cluster immediately
func (m *mockScheduler) SetSchedule(_ context.Context, _ string, _ k8upv1.ScheduleDefinition, fn func(_ context.Context)) error {
	fn(context.TODO())
	return nil
}

func TestCreateJobList_mergePodConfig(t *testing.T) {
	tests := []struct {
		name              string
		jobConfigRef      string
		scheduleConfigRef string
		wantRef           string
	}{
		{
			name:         "GivenJobTemplate_ThenExpectTemplate",
			jobConfigRef: "jobRef",
			wantRef:      "jobRef",
		},
		{
			name:              "GivenScheduleTemplate_ThenExpectTemplate",
			scheduleConfigRef: "scheduleRef",
			wantRef:           "scheduleRef",
		},
		{
			name:              "GivenBothTemplates_ThenExpectPodHasPrecedence",
			scheduleConfigRef: "scheduleRef",
			jobConfigRef:      "jobRef",
			wantRef:           "jobRef",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			scheme := runtime.NewScheme()
			assert.NoError(t, clientgoscheme.AddToScheme(scheme))
			assert.NoError(t, k8upv1.AddToScheme(scheme))

			fclient := fake.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: nsName}}).
				Build()

			var jobConfigRef *corev1.LocalObjectReference
			var scheduleConfigRef *corev1.LocalObjectReference

			if tt.jobConfigRef != "" {
				jobConfigRef = &corev1.LocalObjectReference{Name: tt.jobConfigRef}
			}

			if tt.scheduleConfigRef != "" {
				scheduleConfigRef = &corev1.LocalObjectReference{Name: tt.scheduleConfigRef}
			}

			schedule := ScheduleHandler{
				Config: job.Config{Client: fclient},
				schedule: &k8upv1.Schedule{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "schedule",
						Namespace: nsName,
					},
					Spec: k8upv1.ScheduleSpec{
						PodConfigRef: scheduleConfigRef,
						Backup: &k8upv1.BackupSchedule{
							ScheduleCommon: &k8upv1.ScheduleCommon{
								Schedule: "* * * * *",
							},
							BackupSpec: k8upv1.BackupSpec{
								RunnableSpec: k8upv1.RunnableSpec{
									PodConfigRef: jobConfigRef,
								},
							},
						},
					}}}

			sched := &mockScheduler{}
			assert.NoError(t, schedule.createJobList(context.TODO(), sched))

			backups := &k8upv1.BackupList{}
			assert.NoError(t, fclient.List(context.TODO(), backups, &client.ListOptions{Namespace: nsName}))
			assert.Equal(t, backups.Items[0].Spec.PodConfigRef.Name, tt.wantRef)
		})
	}
}

func Test_generateName(t *testing.T) {
	tests := map[string]struct {
		jobType        k8upv1.JobType
		prefix         string
		expectedPrefix string
	}{
		"GivenShortPrefix_WhenGenerate_ThenUseFullPrefix": {
			jobType:        k8upv1.ArchiveType,
			prefix:         "my-schedule",
			expectedPrefix: "my-schedule-archive-",
		},
		"GivenLongPrefix_WhenGenerate_ThenShortenPrefix": {
			jobType:        k8upv1.ArchiveType,
			prefix:         "my-schedule-with-a-really-long-name-that-could-clash-with-max-length",
			expectedPrefix: "my-schedule-with-a-really-long-name-that-could-cl-archive-",
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			name := generateName(tt.jobType, tt.prefix)
			assert.Contains(t, name, tt.expectedPrefix)
			assert.LessOrEqual(t, len(name), 63)
			assert.Equal(t, len(name), len(tt.expectedPrefix)+5)
		})
	}
}

func newS3Backend(endpoint, bucket string) k8upv1.Backend {
	return k8upv1.Backend{
		S3: &k8upv1.S3Spec{
			Endpoint: endpoint,
			Bucket:   bucket,
		},
	}
}

type builderPodSecurityContext struct {
	corev1.PodSecurityContext
}

func newBuilderPSC() *builderPodSecurityContext {
	return &builderPodSecurityContext{}
}

func (b *builderPodSecurityContext) seLinuxOptions(level string) *builderPodSecurityContext {
	b.SELinuxOptions = &corev1.SELinuxOptions{Level: level}
	return b
}

func (b *builderPodSecurityContext) runAsUser(runAsUser int64) *builderPodSecurityContext {
	b.RunAsUser = &runAsUser
	return b
}

func (b *builderPodSecurityContext) runAsGroup(runAsGroup int64) *builderPodSecurityContext {
	b.RunAsGroup = &runAsGroup
	return b
}

func (b *builderPodSecurityContext) get() *corev1.PodSecurityContext {
	return &b.PodSecurityContext
}
