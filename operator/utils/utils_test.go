package utils

import (
	k8upv1 "github.com/k8up-io/k8up/v2/api/v1"
	corev1 "k8s.io/api/core/v1"
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/utils/ptr"
)

func Test_RandomStringGenerator(t *testing.T) {
	type args struct {
		n int
	}

	tests := []struct {
		name string
		args args
		want int
	}{
		{
			name: "return random string with length zero",
			args: args{n: 0},
		},
		{
			name: "return random string with length one",
			args: args{n: 1},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Lenf(t, RandomStringGenerator(tt.args.n), tt.args.n, "RandomStringGenerator(%v)", tt.args.n)
		})
	}
}

func Test_ZeroLen(t *testing.T) {
	type args struct {
		v interface{}
	}

	type sd struct {
		StrPtrNil     *string
		StrPtrEmpty   *string
		StrPtrFill    *string
		StrEmpty      string
		StrFill       string
		SlicePtrNil   *[]string
		SlicePtrEmpty *[]string
		SlicePtrFill  *[]string
		SliceEmpty    []string
		SliceFill     []string
		MapPtrNil     *map[string]string
		MapPtrEmpty   *map[string]string
		MapPtrFill    *map[string]string
		MapEmpty      map[string]string
		MapFill       map[string]string
		ArrayPtrNil   *[1]string
		ArrayPtrEmpty *[1]string
		ArrayPtrFill  *[1]string
		ArrayEmpty    [1]string
		ArrayFill     [1]string
		IntPtrNil     *int
		IntPtrEmpty   *int
		IntPtrFill    *int
	}
	s := sd{
		StrPtrEmpty:   ptr.To(""),
		StrPtrFill:    ptr.To("this-is-test"),
		StrFill:       "this-is-test",
		SlicePtrEmpty: ptr.To([]string{}),
		SlicePtrFill:  ptr.To([]string{""}),
		SliceFill:     []string{""},
		MapPtrEmpty:   ptr.To(map[string]string{}),
		MapPtrFill:    ptr.To(map[string]string{"": ""}),
		MapFill:       map[string]string{"": ""},
		ArrayPtrEmpty: ptr.To([1]string{}),
		ArrayPtrFill:  ptr.To([1]string{"this-is-test"}),
		ArrayFill:     [1]string{"this-is-test"},
		IntPtrEmpty:   ptr.To(0),
		IntPtrFill:    ptr.To(12),
	}

	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "return true when value is nil",
			args: args{v: nil},
			want: true,
		},
		{
			name: "return true when value is nil string pointer",
			args: args{v: s.StrPtrNil},
			want: true,
		},
		{
			name: "return true when value is empty string pointer",
			args: args{v: s.StrPtrEmpty},
			want: true,
		},
		{
			name: "return false when value is not empty string pointer",
			args: args{v: s.StrPtrFill},
			want: false,
		},
		{
			name: "return true when value is empty string",
			args: args{v: s.StrEmpty},
			want: true,
		},
		{
			name: "return false when value is not empty string",
			args: args{v: s.StrFill},
			want: false,
		},
		{
			name: "return true when value is nil slice pointer",
			args: args{v: s.SlicePtrNil},
			want: true,
		},
		{
			name: "return true when value is empty slice pointer",
			args: args{v: s.SlicePtrEmpty},
			want: true,
		},
		{
			name: "return false when value is not empty slice pointer",
			args: args{v: s.SlicePtrFill},
			want: false,
		},
		{
			name: "return true when value is empty slice",
			args: args{v: s.SliceEmpty},
			want: true,
		},
		{
			name: "return false when value is not empty slice",
			args: args{v: s.SliceFill},
			want: false,
		},
		{
			name: "return true when value is nil map pointer",
			args: args{v: s.MapPtrNil},
			want: true,
		},
		{
			name: "return true when value is empty map pointer",
			args: args{v: s.MapPtrEmpty},
			want: true,
		},
		{
			name: "return false when value is not empty map pointer",
			args: args{v: s.MapPtrFill},
			want: false,
		},
		{
			name: "return true when value is empty map",
			args: args{v: s.MapEmpty},
			want: true,
		},
		{
			name: "return false when value is not empty map",
			args: args{v: s.MapFill},
			want: false,
		},
		{
			name: "return true when value is nil array pointer",
			args: args{v: s.ArrayPtrNil},
			want: true,
		},
		{
			name: "return true when value is empty array pointer",
			args: args{v: s.ArrayPtrEmpty},
			want: true,
		},
		{
			name: "return false when value is not empty array pointer",
			args: args{v: s.ArrayPtrFill},
			want: false,
		},
		{
			name: "return true when value is empty array",
			args: args{v: s.ArrayEmpty},
			want: true,
		},
		{
			name: "return false when value is not empty array",
			args: args{v: s.ArrayFill},
			want: false,
		},
		{
			name: "return true when value is nil int pointer",
			args: args{v: s.IntPtrNil},
			want: true,
		},
		{
			name: "return true when value is empty int pointer",
			args: args{v: s.IntPtrEmpty},
			want: true,
		},
		{
			name: "return false when value is not empty int pointer",
			args: args{v: s.IntPtrFill},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equalf(t, tt.want, ZeroLen(tt.args.v), "ZeroLen(%v)", tt.args.v)
		})
	}
}

func Test_AppendTLSOptionsArgs(t *testing.T) {
	type args struct {
		opts          *k8upv1.TLSOptions
		prefixArgName []string
	}

	tests := []struct {
		name string
		args args
		want []string
	}{
		{
			name: "return empty args when tlsOptions is nil",
			args: args{},
			want: []string(nil),
		},
		{
			name: "return empty args when tlsOptions is nil (with prefix)",
			args: args{prefixArgName: []string{"restore"}},
			want: []string(nil),
		},
		{
			name: "return args with caCert when tlsOptions has property caCert",
			args: args{opts: &k8upv1.TLSOptions{CACert: "/path/of/ca.cert"}},
			want: []string{"-caCert", "/path/of/ca.cert"},
		},
		{
			name: "return args with caCert when tlsOptions has property caCert (with prefix)",
			args: args{
				opts:          &k8upv1.TLSOptions{CACert: "/path/of/ca.cert"},
				prefixArgName: []string{"restore"},
			},
			want: []string{"-restoreCaCert", "/path/of/ca.cert"},
		},
		{
			name: "return args with caCert when tlsOptions has property caCert and pick last index of prefix",
			args: args{
				opts:          &k8upv1.TLSOptions{CACert: "/path/of/ca.cert"},
				prefixArgName: []string{"restore0", "restore1", "restore2"},
			},
			want: []string{"-restore2CaCert", "/path/of/ca.cert"},
		},
		{
			name: "return args with caCert when tlsOptions have properties caCert, clientCert",
			args: args{
				opts: &k8upv1.TLSOptions{
					CACert:     "/path/of/ca.cert",
					ClientCert: "/path/of/client.crt",
				},
			},
			want: []string{"-caCert", "/path/of/ca.cert"},
		},
		{
			name: "return args with caCert when tlsOptions have properties caCert, clientCert (with prefix)",
			args: args{
				opts: &k8upv1.TLSOptions{
					CACert:     "/path/of/ca.cert",
					ClientCert: "/path/of/client.crt",
				},
				prefixArgName: []string{"restore"},
			},
			want: []string{"-restoreCaCert", "/path/of/ca.cert"},
		},
		{
			name: "return args with caCert when tlsOptions have properties caCert, clientKey",
			args: args{
				opts: &k8upv1.TLSOptions{
					CACert:    "/path/of/ca.cert",
					ClientKey: "/path/of/client.key",
				},
			},
			want: []string{"-caCert", "/path/of/ca.cert"},
		},
		{
			name: "return args with caCert when tlsOptions have properties caCert, clientKey (with prefix)",
			args: args{
				opts: &k8upv1.TLSOptions{
					CACert:    "/path/of/ca.cert",
					ClientKey: "/path/of/client.key",
				},
				prefixArgName: []string{"restore"},
			},
			want: []string{"-restoreCaCert", "/path/of/ca.cert"},
		},
		{
			name: "return args with caCert when tlsOptions have properties caCert, clientCert, clientKey",
			args: args{
				opts: &k8upv1.TLSOptions{
					CACert:     "/path/of/ca.cert",
					ClientCert: "/path/of/client.crt",
					ClientKey:  "/path/of/client.key",
				},
			},
			want: []string{"-caCert", "/path/of/ca.cert", "-clientCert", "/path/of/client.crt", "-clientKey", "/path/of/client.key"},
		},
		{
			name: "return args with caCert when tlsOptions have properties caCert, clientCert, clientKey (with prefix)",
			args: args{
				opts: &k8upv1.TLSOptions{
					CACert:     "/path/of/ca.cert",
					ClientCert: "/path/of/client.crt",
					ClientKey:  "/path/of/client.key",
				},
				prefixArgName: []string{"restore"},
			},
			want: []string{"-restoreCaCert", "/path/of/ca.cert", "-restoreClientCert", "/path/of/client.crt", "-restoreClientKey", "/path/of/client.key"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equalf(t, tt.want, AppendTLSOptionsArgs(tt.args.opts, tt.args.prefixArgName...), "AppendTLSOptionsArgs(%v, %v)", tt.args.opts, tt.args.prefixArgName)
		})
	}
}

func Test_AttachTLSVolumes(t *testing.T) {
	type args struct {
		volumes *[]k8upv1.RunnableVolumeSpec
	}

	tests := []struct {
		name string
		args args
		want []corev1.Volume
	}{
		{
			name: "return volumes contain k8up volume when volumes arg is nil",
			args: args{},
			want: []corev1.Volume{
				{
					Name:         _dataDirName,
					VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}},
				},
				{
					Name:         _resticTmpDirName,
					VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}},
				},
				{
					Name:         _resticCacheDirName,
					VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}},
				},
			},
		},
		{
			name: "return volumes contain k8up volume when volumes arg is empty",
			args: args{
				volumes: &[]k8upv1.RunnableVolumeSpec{},
			},
			want: []corev1.Volume{
				{
					Name:         _dataDirName,
					VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}},
				},
				{
					Name:         _resticTmpDirName,
					VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}},
				},
				{
					Name:         _resticCacheDirName,
					VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}},
				},
			},
		},
		{
			name: "return volumes contain k8up volume when volumes arg contains volume with only name",
			args: args{
				volumes: &[]k8upv1.RunnableVolumeSpec{
					{
						Name: "volume",
					},
				},
			},
			want: []corev1.Volume{
				{
					Name:         _dataDirName,
					VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}},
				},
				{
					Name:         _resticTmpDirName,
					VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}},
				},
				{
					Name:         _resticCacheDirName,
					VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}},
				},
			},
		},
		{
			name: "return volumes contain k8up volume and PersistentVolumeClaim",
			args: args{
				volumes: &[]k8upv1.RunnableVolumeSpec{
					{
						Name: "pvc",
						PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
							ClaimName: "claimName",
							ReadOnly:  true,
						},
					},
				},
			},
			want: []corev1.Volume{
				{
					Name:         _dataDirName,
					VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}},
				},
				{
					Name:         _resticTmpDirName,
					VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}},
				},
				{
					Name:         _resticCacheDirName,
					VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}},
				},
				{
					Name: "pvc",
					VolumeSource: corev1.VolumeSource{
						PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
							ClaimName: "claimName",
							ReadOnly:  true,
						},
					},
				},
			},
		},
		{
			name: "return volumes contain k8up volume and SecretVolumeSource",
			args: args{
				volumes: &[]k8upv1.RunnableVolumeSpec{
					{
						Name: "secret",
						Secret: &corev1.SecretVolumeSource{
							SecretName: "secretName",
						},
					},
				},
			},
			want: []corev1.Volume{
				{
					Name:         _dataDirName,
					VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}},
				},
				{
					Name:         _resticTmpDirName,
					VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}},
				},
				{
					Name:         _resticCacheDirName,
					VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}},
				},
				{
					Name: "secret",
					VolumeSource: corev1.VolumeSource{
						Secret: &corev1.SecretVolumeSource{
							SecretName: "secretName",
						},
					},
				},
			},
		},
		{
			name: "return volumes contain k8up volume and ConfigMapVolumeSource",
			args: args{
				volumes: &[]k8upv1.RunnableVolumeSpec{
					{
						Name: "config",
						ConfigMap: &corev1.ConfigMapVolumeSource{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: "configMap",
							},
						},
					},
				},
			},
			want: []corev1.Volume{
				{
					Name:         _dataDirName,
					VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}},
				},
				{
					Name:         _resticTmpDirName,
					VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}},
				},
				{
					Name:         _resticCacheDirName,
					VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}},
				},
				{
					Name: "config",
					VolumeSource: corev1.VolumeSource{
						ConfigMap: &corev1.ConfigMapVolumeSource{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: "configMap",
							},
						},
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equalf(t, tt.want, AttachTLSVolumes(tt.args.volumes), "AttachTLSVolumes(%v)", tt.args.volumes)
		})
	}
}

func Test_AttachTLSVolumeMounts(t *testing.T) {
	k8upPath := "/k8up"
	resticTmpPath := "/tmp"
	resticCachePath := "/.cache"
	type args struct {
		k8upPodVarDir      string
		volumeMounts       *[]corev1.VolumeMount
		addNilVolumeMounts bool
	}

	tests := []struct {
		name string
		args args
		want []corev1.VolumeMount
	}{
		{
			name: "return volume mounts contain k8up mount when volume mounts arg is nil",
			args: args{k8upPodVarDir: k8upPath},
			want: []corev1.VolumeMount{
				{
					Name:      _dataDirName,
					MountPath: k8upPath,
				},
				{
					Name:      _resticTmpDirName,
					MountPath: resticTmpPath,
				},
				{
					Name:      _resticCacheDirName,
					MountPath: resticCachePath,
				},
			},
		},
		{
			name: "return volume mounts contain k8up mount when volume mounts arg is empty",
			args: args{k8upPodVarDir: k8upPath, volumeMounts: &[]corev1.VolumeMount{}},
			want: []corev1.VolumeMount{
				{
					Name:      _dataDirName,
					MountPath: k8upPath,
				},
				{
					Name:      _resticTmpDirName,
					MountPath: resticTmpPath,
				},
				{
					Name:      _resticCacheDirName,
					MountPath: resticCachePath,
				},
			},
		},
		{
			name: "return volume mounts contain k8up mount when call with more volume mounts",
			args: args{
				k8upPodVarDir:      k8upPath,
				volumeMounts:       &[]corev1.VolumeMount{},
				addNilVolumeMounts: true,
			},
			want: []corev1.VolumeMount{
				{
					Name:      _dataDirName,
					MountPath: k8upPath,
				},
				{
					Name:      _resticTmpDirName,
					MountPath: resticTmpPath,
				},
				{
					Name:      _resticCacheDirName,
					MountPath: resticCachePath,
				},
			},
		},
		{
			name: "return volume mounts contain k8up mount and a the one volume is mounted",
			args: args{
				k8upPodVarDir: k8upPath,
				volumeMounts: &[]corev1.VolumeMount{
					{
						Name:      "minio-client-mtls",
						MountPath: "/mnt/tls/",
					},
				},
			},
			want: []corev1.VolumeMount{
				{
					Name:      _dataDirName,
					MountPath: k8upPath,
				},
				{
					Name:      _resticTmpDirName,
					MountPath: resticTmpPath,
				},
				{
					Name:      _resticCacheDirName,
					MountPath: resticCachePath,
				},
				{
					Name:      "minio-client-mtls",
					MountPath: "/mnt/tls/",
				},
			},
		},
		{
			name: "return volume mounts contain k8up mount and a the one volume is mounted (remove duplicate)",
			args: args{
				k8upPodVarDir: k8upPath,
				volumeMounts: &[]corev1.VolumeMount{
					{
						Name:      "minio-client-mtls",
						MountPath: "/mnt/tls/",
					},
					{
						Name:      "minio-client-mtls",
						MountPath: "/mnt/tls/",
					},
				},
			},
			want: []corev1.VolumeMount{
				{
					Name:      _dataDirName,
					MountPath: k8upPath,
				},
				{
					Name:      _resticTmpDirName,
					MountPath: resticTmpPath,
				},
				{
					Name:      _resticCacheDirName,
					MountPath: resticCachePath,
				},
				{
					Name:      "minio-client-mtls",
					MountPath: "/mnt/tls/",
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.args.addNilVolumeMounts {
				assert.Equalf(t, tt.want, AttachTLSVolumeMounts(tt.args.k8upPodVarDir, tt.args.volumeMounts, nil), "Test_AttachTLSVolumeMounts(%v, %v, %v)", tt.args.k8upPodVarDir, tt.args.volumeMounts, nil)
			} else if tt.args.volumeMounts != nil {
				assert.Equalf(t, tt.want, AttachTLSVolumeMounts(tt.args.k8upPodVarDir, tt.args.volumeMounts), "Test_AttachTLSVolumeMounts(%v, %v)", tt.args.k8upPodVarDir, tt.args.volumeMounts)
			} else {
				assert.Equalf(t, tt.want, AttachTLSVolumeMounts(tt.args.k8upPodVarDir), "Test_AttachTLSVolumeMounts(%v)", tt.args.k8upPodVarDir)
			}
		})
	}
}
