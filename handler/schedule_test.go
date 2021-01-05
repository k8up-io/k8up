package handler

import (
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vshn/k8up/api/v1alpha1"
	"github.com/vshn/k8up/cfg"
	"github.com/vshn/k8up/job"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"testing"
)

func TestScheduleHandler_mergeResourcesWithDefaults(t *testing.T) {
	tests := []struct {
		name                        string
		globalCPUResourceLimit      string
		globalCPUResourceRequest    string
		globalMemoryResourceLimit   string
		globalMemoryResourceRequest string
		template                    v1.ResourceRequirements
		resources                   v1.ResourceRequirements
		expected                    v1.ResourceRequirements
	}{
		{
			name:     "Given_NoGlobalDefaults_And_NoScheduleDefaults_When_NoSpec_Then_LeaveEmpty",
			expected: v1.ResourceRequirements{},
		},
		{
			name: "Given_NoGlobalDefaults_And_NoScheduleDefaults_When_Spec_Then_UseSpec",
			resources: v1.ResourceRequirements{
				Requests: v1.ResourceList{
					v1.ResourceCPU: resource.MustParse("50m"),
				},
			},
			expected: v1.ResourceRequirements{
				Requests: v1.ResourceList{
					v1.ResourceCPU: resource.MustParse("50m"),
				},
			},
		},
		{
			name: "Given_NoGlobalDefaults_And_ScheduleDefaults_When_NoSpec_Then_ApplyScheduleDefaults",
			template: v1.ResourceRequirements{
				Limits: v1.ResourceList{
					v1.ResourceCPU: resource.MustParse("200m"),
				},
			},
			expected: v1.ResourceRequirements{
				Limits: v1.ResourceList{
					v1.ResourceCPU: resource.MustParse("200m"),
				},
			},
		},
		{
			name: "Given_NoGlobalDefaults_And_ScheduleDefaults_When_Spec_Then_UseSpec",
			template: v1.ResourceRequirements{
				Limits: v1.ResourceList{
					v1.ResourceCPU: resource.MustParse("200m"),
				},
			},
			resources: v1.ResourceRequirements{
				Limits: v1.ResourceList{
					v1.ResourceCPU: resource.MustParse("50m"),
				},
			},
			expected: v1.ResourceRequirements{
				Limits: v1.ResourceList{
					v1.ResourceCPU: resource.MustParse("50m"),
				},
			},
		},
		{
			name:                        "Given_GlobalDefaults_And_NoScheduleDefaults_When_NoSpec_Then_UseGlobalDefaults",
			globalMemoryResourceRequest: "10Mi",
			template: v1.ResourceRequirements{
				Limits: v1.ResourceList{
					v1.ResourceCPU: resource.MustParse("200m"),
				},
			},
			expected: v1.ResourceRequirements{
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
			resources: v1.ResourceRequirements{
				Requests: v1.ResourceList{
					v1.ResourceMemory: resource.MustParse("20Mi"),
				},
			},
			expected: v1.ResourceRequirements{
				Requests: v1.ResourceList{
					v1.ResourceMemory: resource.MustParse("20Mi"),
				},
			},
		},
		{
			name:                   "Given_GlobalDefaults_And_ScheduleDefaults_When_NoSpec_Then_UseSchedule",
			globalCPUResourceLimit: "10m",
			template: v1.ResourceRequirements{
				Limits: v1.ResourceList{
					v1.ResourceCPU: resource.MustParse("200m"),
				},
			},
			expected: v1.ResourceRequirements{
				Limits: v1.ResourceList{
					v1.ResourceCPU: resource.MustParse("200m"),
				},
			},
		},
		{
			name:                   "Given_GlobalDefaults_And_ScheduleDefaults_When_Spec_Then_UseSpec",
			globalCPUResourceLimit: "10m",
			template: v1.ResourceRequirements{
				Limits: v1.ResourceList{
					v1.ResourceCPU: resource.MustParse("100m"),
				},
			},
			resources: v1.ResourceRequirements{
				Limits: v1.ResourceList{
					v1.ResourceCPU: resource.MustParse("200m"),
				},
			},
			expected: v1.ResourceRequirements{
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
			s := ScheduleHandler{schedule: &v1alpha1.Schedule{Spec: v1alpha1.ScheduleSpec{
				ResourceRequirementsTemplate: tt.template,
			}}}
			s.mergeResourcesWithDefaults(&tt.resources)
			assert.Equal(t, tt.expected, tt.resources)
		})
	}
}

func TestScheduleHandler_getEffectiveSchedule(t *testing.T) {
	tests := map[string]struct {
		schedule             *v1alpha1.Schedule
		originalSchedule     string
		expectedStatusUpdate bool
		expectedSchedule     string
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
					EffectiveSchedules: map[v1alpha1.JobType]string{
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
