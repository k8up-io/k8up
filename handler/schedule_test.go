package handler

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"github.com/vshn/k8up/api/v1alpha1"
	"github.com/vshn/k8up/cfg"
	"github.com/vshn/k8up/job"
)

func TestScheduleHandler_mergeResourcesWithDefaults(t *testing.T) {
	tests := map[string]struct {
		globalCPUResourceLimit      string
		globalCPUResourceRequest    string
		globalMemoryResourceLimit   string
		globalMemoryResourceRequest string
		givenScheduleTemplate       v1.ResourceRequirements
		givenResourceTemplate       v1.ResourceRequirements
		expectedTemplate            v1.ResourceRequirements
	}{
		"Given_NoGlobalDefaults_And_NoScheduleDefaults_When_NoSpec_Then_LeaveEmpty": {
			expectedTemplate: v1.ResourceRequirements{},
		},
		"Given_NoGlobalDefaults_And_NoScheduleDefaults_When_Spec_Then_UseSpec": {
			givenResourceTemplate: v1.ResourceRequirements{
				Requests: newCPUResourceList("50m"),
			},
			expectedTemplate: v1.ResourceRequirements{
				Requests: newCPUResourceList("50m"),
			},
		},
		"Given_NoGlobalDefaults_And_ScheduleDefaults_When_NoSpec_Then_ApplyScheduleDefaults": {
			givenScheduleTemplate: v1.ResourceRequirements{
				Limits: newCPUResourceList("200m"),
			},
			expectedTemplate: v1.ResourceRequirements{
				Limits: newCPUResourceList("200m"),
			},
		},
		"Given_NoGlobalDefaults_And_ScheduleDefaults_When_Spec_Then_UseSpec": {
			givenScheduleTemplate: v1.ResourceRequirements{
				Limits: newCPUResourceList("200m"),
			},
			givenResourceTemplate: v1.ResourceRequirements{
				Limits: newCPUResourceList("50m"),
			},
			expectedTemplate: v1.ResourceRequirements{
				Limits: newCPUResourceList("50m"),
			},
		},
		"Given_GlobalDefaults_And_NoScheduleDefaults_When_NoSpec_Then_UseGlobalDefaults": {
			globalMemoryResourceRequest: "10Mi",
			givenScheduleTemplate: v1.ResourceRequirements{
				Limits: newCPUResourceList("200m"),
			},
			expectedTemplate: v1.ResourceRequirements{
				Limits: newCPUResourceList("200m"),
				Requests: v1.ResourceList{
					v1.ResourceMemory: resource.MustParse("10Mi"),
				},
			},
		},
		"Given_GlobalDefaults_And_NoScheduleDefaults_When_Spec_Then_UseSpec": {
			globalMemoryResourceRequest: "10Mi",
			givenResourceTemplate: v1.ResourceRequirements{
				Requests: v1.ResourceList{
					v1.ResourceMemory: resource.MustParse("20Mi"),
				},
			},
			expectedTemplate: v1.ResourceRequirements{
				Requests: v1.ResourceList{
					v1.ResourceMemory: resource.MustParse("20Mi"),
				},
			},
		},
		"Given_GlobalDefaults_And_ScheduleDefaults_When_NoSpec_Then_UseSchedule": {
			globalCPUResourceLimit: "10m",
			givenScheduleTemplate: v1.ResourceRequirements{
				Limits: newCPUResourceList("200m"),
			},
			expectedTemplate: v1.ResourceRequirements{
				Limits: newCPUResourceList("200m"),
			},
		},
		"Given_GlobalDefaults_And_ScheduleDefaults_When_Spec_Then_UseSpec": {
			globalCPUResourceLimit: "10m",
			givenScheduleTemplate: v1.ResourceRequirements{
				Limits: newCPUResourceList("100m"),
			},
			givenResourceTemplate: v1.ResourceRequirements{
				Limits: newCPUResourceList("200m"),
			},
			expectedTemplate: v1.ResourceRequirements{
				Limits: newCPUResourceList("200m"),
			},
		},
	}
	cfg.Config = cfg.NewDefaultConfig()
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			cfg.Config.GlobalCPUResourceLimit = tt.globalCPUResourceLimit
			cfg.Config.GlobalCPUResourceRequest = tt.globalCPUResourceRequest
			cfg.Config.GlobalMemoryResourceLimit = tt.globalMemoryResourceLimit
			cfg.Config.GlobalMemoryResourceRequest = tt.globalMemoryResourceRequest
			require.NoError(t, cfg.Config.ValidateSyntax())
			schedule := ScheduleHandler{schedule: &v1alpha1.Schedule{Spec: v1alpha1.ScheduleSpec{
				ResourceRequirementsTemplate: tt.givenScheduleTemplate,
			}}}
			res := &v1alpha1.RunnableSpec{
				Resources: tt.givenResourceTemplate,
			}
			schedule.mergeResourcesWithDefaults(res)
			assert.Equal(t, tt.expectedTemplate, res.Resources)
		})
	}
}

func newCPUResourceList(amount string) v1.ResourceList {
	return v1.ResourceList{
		v1.ResourceCPU: resource.MustParse(amount),
	}
}

func TestScheduleHandler_mergeBackendWithDefaults(t *testing.T) {
	tests := map[string]struct {
		globalS3Bucket       string
		givenScheduleBackend v1alpha1.Backend
		givenResourceBackend v1alpha1.Backend
		expectedBackend      v1alpha1.Backend
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
	cfg.Config = cfg.NewDefaultConfig()
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			cfg.Config.GlobalS3Bucket = tt.globalS3Bucket
			require.NoError(t, cfg.Config.ValidateSyntax())
			schedule := ScheduleHandler{schedule: &v1alpha1.Schedule{Spec: v1alpha1.ScheduleSpec{
				Backend: &tt.givenScheduleBackend,
			}}}
			res := &v1alpha1.RunnableSpec{
				Backend: &tt.givenResourceBackend,
			}
			schedule.mergeBackendWithDefaults(res)
			assert.NotNil(t, res.Backend.S3)
			assert.Equal(t, *tt.expectedBackend.S3, *res.Backend.S3)
		})
	}
}

func newS3Backend(endpoint, bucket string) v1alpha1.Backend {
	return v1alpha1.Backend{
		S3: &v1alpha1.S3Spec{
			Endpoint: endpoint,
			Bucket:   bucket,
		},
	}
}

func TestScheduleHandler_getEffectiveSchedule(t *testing.T) {
	tests := map[string]struct {
		schedule             *v1alpha1.Schedule
		originalSchedule     v1alpha1.ScheduleDefinition
		expectedStatusUpdate bool
		expectedSchedule     v1alpha1.ScheduleDefinition
	}{
		"GivenScheduleWithoutStatus_WhenUsingRandomSchedule_ThenPutGeneratedScheduleInStatus": {
			schedule: &v1alpha1.Schedule{
				Spec: v1alpha1.ScheduleSpec{
					Backup: &v1alpha1.BackupSchedule{},
				},
			},
			originalSchedule:     "@hourly-random",
			expectedSchedule:     "26 * * * *",
			expectedStatusUpdate: true,
		},
		"GivenScheduleWithStatus_WhenUsingRandomSchedule_ThenUseGeneratedScheduleFromStatus": {
			schedule: &v1alpha1.Schedule{
				Spec: v1alpha1.ScheduleSpec{
					Backup: &v1alpha1.BackupSchedule{},
				},
				Status: v1alpha1.ScheduleStatus{
					EffectiveSchedules: map[v1alpha1.JobType]v1alpha1.ScheduleDefinition{
						v1alpha1.BackupType: "26 * 3 * *",
					},
				},
			},
			originalSchedule: "@hourly-random",
			expectedSchedule: "26 * 3 * *",
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			s := &ScheduleHandler{
				schedule: tt.schedule,
				Config:   job.Config{Log: zap.New(zap.UseDevMode(true))},
			}
			result := s.getEffectiveSchedule(v1alpha1.BackupType, tt.originalSchedule)
			assert.Equal(t, tt.expectedSchedule, result)
			assert.Equal(t, tt.expectedStatusUpdate, s.requireStatusUpdate)
			if tt.expectedStatusUpdate {
				assert.Equal(t, tt.expectedSchedule, tt.schedule.Status.EffectiveSchedules[v1alpha1.BackupType])
			}
		})
	}
}
