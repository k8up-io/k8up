package backupcontroller

import (
	"testing"

	k8upv1 "github.com/k8up-io/k8up/v2/api/v1"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Test_findNode(t *testing.T) {
	type args struct {
		pv  *corev1.PersistentVolume
		pvc corev1.PersistentVolumeClaim
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "returns node when PV affinity is set",
			args: args{
				pv: &corev1.PersistentVolume{
					Spec: corev1.PersistentVolumeSpec{
						NodeAffinity: &corev1.VolumeNodeAffinity{
							Required: &corev1.NodeSelector{
								NodeSelectorTerms: []corev1.NodeSelectorTerm{
									{
										MatchExpressions: []corev1.NodeSelectorRequirement{
											{
												Key:      corev1.LabelHostname,
												Operator: corev1.NodeSelectorOpIn,
												Values:   []string{"node-a"},
											},
										},
									},
								},
							},
						},
					},
				},
				pvc: corev1.PersistentVolumeClaim{},
			},
			want: "node-a",
		},
		{
			name: "returns empty string when wrong PV affinity operator is set",
			args: args{
				pv: &corev1.PersistentVolume{
					Spec: corev1.PersistentVolumeSpec{
						NodeAffinity: &corev1.VolumeNodeAffinity{
							Required: &corev1.NodeSelector{
								NodeSelectorTerms: []corev1.NodeSelectorTerm{
									{
										MatchExpressions: []corev1.NodeSelectorRequirement{
											{
												Key:      corev1.LabelHostname,
												Operator: corev1.NodeSelectorOpExists,
												Values:   []string{"node-a"},
											},
										},
									},
								},
							},
						},
					},
				},
				pvc: corev1.PersistentVolumeClaim{},
			},
			want: "",
		},
		{
			name: "returns empty string when wrong PV affinity key is set",
			args: args{
				pv: &corev1.PersistentVolume{
					Spec: corev1.PersistentVolumeSpec{
						NodeAffinity: &corev1.VolumeNodeAffinity{
							Required: &corev1.NodeSelector{
								NodeSelectorTerms: []corev1.NodeSelectorTerm{
									{
										MatchExpressions: []corev1.NodeSelectorRequirement{
											{
												Key:      "hostname",
												Operator: corev1.NodeSelectorOpIn,
												Values:   []string{"node-a"},
											},
										},
									},
								},
							},
						},
					},
				},
				pvc: corev1.PersistentVolumeClaim{},
			},
			want: "",
		},
		{
			name: "returns node when no affinity is set but annotation on PVC",
			args: args{
				pv: &corev1.PersistentVolume{},
				pvc: corev1.PersistentVolumeClaim{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: map[string]string{k8upv1.AnnotationK8upHostname: "node-a"},
					},
				},
			},
			want: "node-a",
		},
		{
			name: "returns empty string when no affinity is set and invalid annotation on PVC",
			args: args{
				pv: &corev1.PersistentVolume{},
				pvc: corev1.PersistentVolumeClaim{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: map[string]string{"hostname": "node-a"},
					},
				},
			},
			want: "",
		},
		{
			name: "returns empty string if neither affinity nor annotation is set",
			args: args{
				pv:  &corev1.PersistentVolume{},
				pvc: corev1.PersistentVolumeClaim{},
			},
			want: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equalf(t, tt.want, findNode(tt.args.pv, tt.args.pvc), "findNode(%v, %v)", tt.args.pv, tt.args.pvc)
		})
	}
}
