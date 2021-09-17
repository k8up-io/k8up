package handler

import (
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"

	k8upv1 "github.com/vshn/k8up/api/v1"
	"github.com/vshn/k8up/operator/cfg"
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
	cfg.Config.OperatorNamespace = "irrelevant-but-required"
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
	cfg.Config.OperatorNamespace = "irrelevant-but-required"
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

func newS3Backend(endpoint, bucket string) k8upv1.Backend {
	return k8upv1.Backend{
		S3: &k8upv1.S3Spec{
			Endpoint: endpoint,
			Bucket:   bucket,
		},
	}
}
