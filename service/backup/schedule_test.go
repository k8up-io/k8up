package backup

import (
	"reflect"
	"sync"
	"testing"

	backupv1alpha1 "git.vshn.net/vshn/baas/apis/backup/v1alpha1"
	baas8scli "git.vshn.net/vshn/baas/client/k8s/clientset/versioned"
	"git.vshn.net/vshn/baas/log"
	cron "github.com/Infowatch/cron"
	applogger "github.com/spotahome/kooper/log"
	"k8s.io/api/core/v1"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	testclient "k8s.io/client-go/kubernetes/fake"
)

func TestPVCBackupper_SameSpec(t *testing.T) {
	type fields struct {
		backup      *backupv1alpha1.Backup
		k8sCLI      kubernetes.Interface
		baasCLI     baas8scli.Interface
		log         log.Logger
		running     bool
		mutex       sync.Mutex
		cron        *cron.Cron
		cronID      cron.EntryID
		checkCronID cron.EntryID
	}
	type args struct {
		baas *backupv1alpha1.Backup
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   bool
	}{
		{
			name: "SameSpec true",
			fields: fields{
				backup: &backupv1alpha1.Backup{
					Spec: backupv1alpha1.BackupSpec{
						Schedule: "* * * *",
					},
				},
			},
			args: args{
				baas: &backupv1alpha1.Backup{
					Spec: backupv1alpha1.BackupSpec{
						Schedule: "* * * *",
					},
				},
			},
			want: true,
		},
		{
			name: "SameSpec false",
			fields: fields{
				backup: &backupv1alpha1.Backup{
					Spec: backupv1alpha1.BackupSpec{
						Schedule: "* * * *",
					},
				},
			},
			args: args{
				baas: &backupv1alpha1.Backup{
					Spec: backupv1alpha1.BackupSpec{
						Schedule: "*  * *",
					},
				},
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &PVCBackupper{
				backup:      tt.fields.backup,
				k8sCLI:      tt.fields.k8sCLI,
				baasCLI:     tt.fields.baasCLI,
				log:         tt.fields.log,
				running:     tt.fields.running,
				mutex:       tt.fields.mutex,
				cron:        tt.fields.cron,
				cronID:      tt.fields.cronID,
				checkCronID: tt.fields.checkCronID,
			}
			if got := p.SameSpec(tt.args.baas); got != tt.want {
				t.Errorf("PVCBackupper.SameSpec() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPVCBackupper_podErrors(t *testing.T) {
	type fields struct {
		backup      *backupv1alpha1.Backup
		k8sCLI      kubernetes.Interface
		baasCLI     baas8scli.Interface
		log         log.Logger
		running     bool
		mutex       sync.Mutex
		finishedC   chan bool
		cron        *cron.Cron
		cronID      cron.EntryID
		checkCronID cron.EntryID
	}
	type args struct {
		filter string
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   bool
	}{
		{
			name: "No pods",
			fields: fields{
				k8sCLI: testclient.NewSimpleClientset(),
				backup: &backupv1alpha1.Backup{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test",
					},
				},
			},
			args: args{
				filter: "dummy",
			},
			want: true,
		},
		{
			name: "1 pod no fail",
			fields: fields{
				k8sCLI: testclient.NewSimpleClientset(),
				backup: &backupv1alpha1.Backup{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test",
					},
				},
			},
			args: args{
				filter: "backupPod=true",
			},
			want: false,
		},
		{
			name: "1 pod fail",
			fields: fields{
				k8sCLI: testclient.NewSimpleClientset(),
				backup: &backupv1alpha1.Backup{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test",
					},
				},
			},
			args: args{
				filter: "backupPod=true",
			},
			want: true,
		},
		{
			name: "1 restarts",
			fields: fields{
				k8sCLI: testclient.NewSimpleClientset(),
				backup: &backupv1alpha1.Backup{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test",
					},
				},
			},
			args: args{
				filter: "backupPod=true",
			},
			want: true,
		},
	}

	tests[1].fields.k8sCLI.CoreV1().Pods("").Create(
		&v1.Pod{
			TypeMeta: metav1.TypeMeta{
				Kind:       "Pod",
				APIVersion: "v1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					"backupPod": "true",
				},
			},
		})
	tests[2].fields.k8sCLI.CoreV1().Pods("").Create(
		&v1.Pod{
			TypeMeta: metav1.TypeMeta{
				Kind:       "Pod",
				APIVersion: "v1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					"backupPod": "true",
				},
			},
			Status: apiv1.PodStatus{
				ContainerStatuses: []apiv1.ContainerStatus{
					{
						State: apiv1.ContainerState{
							Waiting: &apiv1.ContainerStateWaiting{
								Message: "error",
							},
						},
					},
				},
			},
		})

	tests[3].fields.k8sCLI.CoreV1().Pods("").Create(
		&v1.Pod{
			TypeMeta: metav1.TypeMeta{
				Kind:       "Pod",
				APIVersion: "v1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					"backupPod": "true",
				},
			},
			Status: apiv1.PodStatus{
				ContainerStatuses: []apiv1.ContainerStatus{
					{
						RestartCount: 3,
					},
				},
			},
		})

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &PVCBackupper{
				backup:      tt.fields.backup,
				k8sCLI:      tt.fields.k8sCLI,
				baasCLI:     tt.fields.baasCLI,
				log:         tt.fields.log,
				running:     tt.fields.running,
				mutex:       tt.fields.mutex,
				cron:        tt.fields.cron,
				cronID:      tt.fields.cronID,
				checkCronID: tt.fields.checkCronID,
			}
			if got := p.podErrors(tt.args.filter); got != tt.want {
				t.Errorf("PVCBackupper.podErrors() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPVCBackupper_listPVCs(t *testing.T) {
	type fields struct {
		backup      *backupv1alpha1.Backup
		k8sCLI      kubernetes.Interface
		baasCLI     baas8scli.Interface
		log         log.Logger
		running     bool
		mutex       sync.Mutex
		finishedC   chan bool
		cron        *cron.Cron
		cronID      cron.EntryID
		checkCronID cron.EntryID
		config      config
	}
	type args struct {
		annotation string
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   []apiv1.Volume
	}{
		{
			name: "No PVCs",
			fields: fields{
				log:    &applogger.Std{},
				k8sCLI: testclient.NewSimpleClientset(),
				backup: &backupv1alpha1.Backup{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test",
						Namespace: "test",
					},
				},
			},
			args: args{
				annotation: "test",
			},
			want: []apiv1.Volume{},
		},
		{
			name: "1 PVC without annotation",
			fields: fields{
				log:    &applogger.Std{},
				k8sCLI: testclient.NewSimpleClientset(),
				backup: &backupv1alpha1.Backup{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test",
						Namespace: "test",
					},
				},
			},
			args: args{
				annotation: "test",
			},
			want: []apiv1.Volume{
				{
					Name: "testclaim",
					VolumeSource: apiv1.VolumeSource{
						PersistentVolumeClaim: &apiv1.PersistentVolumeClaimVolumeSource{
							ClaimName: "testclaim",
							ReadOnly:  true,
						},
					},
				},
			},
		},
		{
			name: "1 PVC with annotation ignore",
			fields: fields{
				log:    &applogger.Std{},
				k8sCLI: testclient.NewSimpleClientset(),
				backup: &backupv1alpha1.Backup{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test",
						Namespace: "test",
					},
				},
			},
			args: args{
				annotation: "test",
			},
			want: []apiv1.Volume{},
		},
		{
			name: "1 PVC RWO with annotation",
			fields: fields{
				log:    &applogger.Std{},
				k8sCLI: testclient.NewSimpleClientset(),
				backup: &backupv1alpha1.Backup{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test",
						Namespace: "test",
					},
				},
			},
			args: args{
				annotation: "test",
			},
			want: []apiv1.Volume{
				{
					Name: "testclaim",
					VolumeSource: apiv1.VolumeSource{
						PersistentVolumeClaim: &apiv1.PersistentVolumeClaimVolumeSource{
							ClaimName: "testclaim",
							ReadOnly:  true,
						},
					},
				},
			},
		},
	}

	tests[1].fields.k8sCLI.CoreV1().PersistentVolumeClaims(tests[1].fields.backup.Namespace).Create(
		&apiv1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name: "testclaim",
			},
			Spec: apiv1.PersistentVolumeClaimSpec{
				VolumeName: "testvol",
				AccessModes: []apiv1.PersistentVolumeAccessMode{
					"ReadWriteMany",
				},
			},
		})

	tests[1].fields.k8sCLI.CoreV1().PersistentVolumes().Create(
		&apiv1.PersistentVolume{
			ObjectMeta: metav1.ObjectMeta{
				Name: "testvol",
			},
			Spec: apiv1.PersistentVolumeSpec{
				PersistentVolumeSource: apiv1.PersistentVolumeSource{
					HostPath: &apiv1.HostPathVolumeSource{
						Path: "test",
					},
				},
			},
		},
	)

	tests[2].fields.k8sCLI.CoreV1().PersistentVolumeClaims(tests[1].fields.backup.Namespace).Create(
		&apiv1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name: "testclaim",
				Annotations: map[string]string{
					"test": "don't",
				},
			},
			Spec: apiv1.PersistentVolumeClaimSpec{
				VolumeName: "testvol",
				AccessModes: []apiv1.PersistentVolumeAccessMode{
					"ReadWriteMany",
				},
			},
		})

	tests[2].fields.k8sCLI.CoreV1().PersistentVolumes().Create(
		&apiv1.PersistentVolume{
			ObjectMeta: metav1.ObjectMeta{
				Name: "testvol",
			},
			Spec: apiv1.PersistentVolumeSpec{
				PersistentVolumeSource: apiv1.PersistentVolumeSource{
					HostPath: &apiv1.HostPathVolumeSource{
						Path: "test",
					},
				},
			},
		},
	)

	tests[3].fields.k8sCLI.CoreV1().PersistentVolumeClaims(tests[1].fields.backup.Namespace).Create(
		&apiv1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name: "testclaim",
				Annotations: map[string]string{
					"test": "true",
				},
			},
			Spec: apiv1.PersistentVolumeClaimSpec{
				VolumeName: "testvol",
				AccessModes: []apiv1.PersistentVolumeAccessMode{
					"ReadWriteOnce",
				},
			},
		})

	tests[3].fields.k8sCLI.CoreV1().PersistentVolumes().Create(
		&apiv1.PersistentVolume{
			ObjectMeta: metav1.ObjectMeta{
				Name: "testvol",
			},
			Spec: apiv1.PersistentVolumeSpec{
				PersistentVolumeSource: apiv1.PersistentVolumeSource{
					HostPath: &apiv1.HostPathVolumeSource{
						Path: "test",
					},
				},
			},
		},
	)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &PVCBackupper{
				backup:      tt.fields.backup,
				k8sCLI:      tt.fields.k8sCLI,
				baasCLI:     tt.fields.baasCLI,
				log:         tt.fields.log,
				running:     tt.fields.running,
				mutex:       tt.fields.mutex,
				cron:        tt.fields.cron,
				cronID:      tt.fields.cronID,
				checkCronID: tt.fields.checkCronID,
				config:      tt.fields.config,
			}
			if got := p.listPVCs(tt.args.annotation); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("PVCBackupper.listPVCs() = %v, want %v", got, tt.want)
			}
		})
	}
}
