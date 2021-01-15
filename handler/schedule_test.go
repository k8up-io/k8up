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
	tests := []struct {
		name                        string
		globalCPUResourceLimit      string
		globalCPUResourceRequest    string
		globalMemoryResourceLimit   string
		globalMemoryResourceRequest string
		givenScheduleTemplate       v1.ResourceRequirements
		givenResourceTemplate       v1.ResourceRequirements
		expectedTemplate            v1.ResourceRequirements
	}{
		{
			name:             "Given_NoGlobalDefaults_And_NoScheduleDefaults_When_NoSpec_Then_LeaveEmpty",
			expectedTemplate: v1.ResourceRequirements{},
		},
		{
			name: "Given_NoGlobalDefaults_And_NoScheduleDefaults_When_Spec_Then_UseSpec",
			givenResourceTemplate: v1.ResourceRequirements{
				Requests: v1.ResourceList{
					v1.ResourceCPU: resource.MustParse("50m"),
				},
			},
			expectedTemplate: v1.ResourceRequirements{
				Requests: v1.ResourceList{
					v1.ResourceCPU: resource.MustParse("50m"),
				},
			},
		},
		{
			name: "Given_NoGlobalDefaults_And_ScheduleDefaults_When_NoSpec_Then_ApplyScheduleDefaults",
			givenScheduleTemplate: v1.ResourceRequirements{
				Limits: v1.ResourceList{
					v1.ResourceCPU: resource.MustParse("200m"),
				},
			},
			expectedTemplate: v1.ResourceRequirements{
				Limits: v1.ResourceList{
					v1.ResourceCPU: resource.MustParse("200m"),
				},
			},
		},
		{
			name: "Given_NoGlobalDefaults_And_ScheduleDefaults_When_Spec_Then_UseSpec",
			givenScheduleTemplate: v1.ResourceRequirements{
				Limits: v1.ResourceList{
					v1.ResourceCPU: resource.MustParse("200m"),
				},
			},
			givenResourceTemplate: v1.ResourceRequirements{
				Limits: v1.ResourceList{
					v1.ResourceCPU: resource.MustParse("50m"),
				},
			},
			expectedTemplate: v1.ResourceRequirements{
				Limits: v1.ResourceList{
					v1.ResourceCPU: resource.MustParse("50m"),
				},
			},
		},
		{
			name:                        "Given_GlobalDefaults_And_NoScheduleDefaults_When_NoSpec_Then_UseGlobalDefaults",
			globalMemoryResourceRequest: "10Mi",
			givenScheduleTemplate: v1.ResourceRequirements{
				Limits: v1.ResourceList{
					v1.ResourceCPU: resource.MustParse("200m"),
				},
			},
			expectedTemplate: v1.ResourceRequirements{
				Limits: v1.ResourceList{
					v1.ResourceCPU: resource.MustParse("200m"),
				},
				Requests: v1.ResourceList{
					v1.ResourceMemory: resource.MustParse("10Mi"),
				},
			},
		},
		{
			name:                        "Given_GlobalDefaults_And_NoScheduleDefaults_When_Spec_Then_UseSpec",
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
		{
			name:                   "Given_GlobalDefaults_And_ScheduleDefaults_When_NoSpec_Then_UseSchedule",
			globalCPUResourceLimit: "10m",
			givenScheduleTemplate: v1.ResourceRequirements{
				Limits: v1.ResourceList{
					v1.ResourceCPU: resource.MustParse("200m"),
				},
			},
			expectedTemplate: v1.ResourceRequirements{
				Limits: v1.ResourceList{
					v1.ResourceCPU: resource.MustParse("200m"),
				},
			},
		},
		{
			name:                   "Given_GlobalDefaults_And_ScheduleDefaults_When_Spec_Then_UseSpec",
			globalCPUResourceLimit: "10m",
			givenScheduleTemplate: v1.ResourceRequirements{
				Limits: v1.ResourceList{
					v1.ResourceCPU: resource.MustParse("100m"),
				},
			},
			givenResourceTemplate: v1.ResourceRequirements{
				Limits: v1.ResourceList{
					v1.ResourceCPU: resource.MustParse("200m"),
				},
			},
			expectedTemplate: v1.ResourceRequirements{
				Limits: v1.ResourceList{
					v1.ResourceCPU: resource.MustParse("200m"),
				},
			},
		},
	}
	cfg.Config = cfg.NewDefaultConfig()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
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

func TestScheduleHandler_mergeBackendWithDefaults(t *testing.T) {
	tests := []struct {
		name                 string
		globalS3Bucket       string
		givenScheduleBackend v1alpha1.Backend
		givenResourceBackend v1alpha1.Backend
		expectedBackend      v1alpha1.Backend
	}{
		{
			name: "Given_NoGlobalDefaults_And_NoScheduleDefaults_When_Spec_Then_UseSpec",
			givenResourceBackend: v1alpha1.Backend{
				S3: &v1alpha1.S3Spec{
					Endpoint: "https://resource-url",
					Bucket:   "resource-bucket",
				},
			},
			expectedBackend: v1alpha1.Backend{
				S3: &v1alpha1.S3Spec{
					Endpoint: "https://resource-url",
					Bucket:   "resource-bucket",
				},
			},
		},
		{
			name: "Given_NoGlobalDefaults_And_ScheduleDefaults_When_NoSpec_Then_ApplyScheduleDefaults",
			givenScheduleBackend: v1alpha1.Backend{
				S3: &v1alpha1.S3Spec{
					Endpoint: "https://schedule-url",
					Bucket:   "schedule-bucket",
				},
			},
			expectedBackend: v1alpha1.Backend{
				S3: &v1alpha1.S3Spec{
					Endpoint: "https://schedule-url",
					Bucket:   "schedule-bucket",
				},
			},
		},
		{
			name: "Given_NoGlobalDefaults_And_ScheduleDefaults_When_Spec_Then_UseSpec",
			givenScheduleBackend: v1alpha1.Backend{
				S3: &v1alpha1.S3Spec{
					Endpoint: "https://schedule-url",
					Bucket:   "schedule-bucket",
				},
			},
			givenResourceBackend: v1alpha1.Backend{
				S3: &v1alpha1.S3Spec{
					Endpoint: "https://resource-url",
					Bucket:   "resource-bucket",
				},
			},
			expectedBackend: v1alpha1.Backend{
				S3: &v1alpha1.S3Spec{
					Endpoint: "https://resource-url",
					Bucket:   "resource-bucket",
				},
			},
		},
		{
			name:           "Given_GlobalDefaults_And_NoScheduleDefaults_When_NoSpec_Then_UseGlobalDefaults",
			globalS3Bucket: "global-bucket",
			givenScheduleBackend: v1alpha1.Backend{
				S3: &v1alpha1.S3Spec{
					Endpoint: "https://schedule-url",
				},
			},
			expectedBackend: v1alpha1.Backend{
				S3: &v1alpha1.S3Spec{
					Endpoint: "https://schedule-url",
					Bucket:   "",
				},
			},
		},
		{
			name:           "Given_GlobalDefaults_And_NoScheduleDefaults_When_Spec_Then_UseSpec",
			globalS3Bucket: "global-bucket",
			givenResourceBackend: v1alpha1.Backend{
				S3: &v1alpha1.S3Spec{
					Endpoint: "https://resource-url",
					Bucket:   "resource-bucket",
				},
			},
			expectedBackend: v1alpha1.Backend{
				S3: &v1alpha1.S3Spec{
					Endpoint: "https://resource-url",
					Bucket:   "resource-bucket",
				},
			},
		},
		{
			name:           "Given_GlobalDefaults_And_ScheduleDefaults_When_NoSpec_Then_UseSchedule",
			globalS3Bucket: "global-bucket",
			givenScheduleBackend: v1alpha1.Backend{
				S3: &v1alpha1.S3Spec{
					Endpoint: "https://schedule-url",
					Bucket:   "schedule-bucket",
				},
			},
			expectedBackend: v1alpha1.Backend{
				S3: &v1alpha1.S3Spec{
					Endpoint: "https://schedule-url",
					Bucket:   "schedule-bucket",
				},
			},
		},
		{
			name:           "Given_GlobalDefaults_And_ScheduleDefaults_When_Spec_Then_UseSpec",
			globalS3Bucket: "global-bucket",
			givenScheduleBackend: v1alpha1.Backend{
				S3: &v1alpha1.S3Spec{
					Endpoint: "https://schedule-url",
					Bucket:   "schedule-bucket",
				},
			},
			givenResourceBackend: v1alpha1.Backend{
				S3: &v1alpha1.S3Spec{
					Endpoint: "https://resource-url",
					Bucket:   "resource-bucket",
				},
			},
			expectedBackend: v1alpha1.Backend{
				S3: &v1alpha1.S3Spec{
					Endpoint: "https://resource-url",
					Bucket:   "resource-bucket",
				},
			},
		},
	}
	cfg.Config = cfg.NewDefaultConfig()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
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
