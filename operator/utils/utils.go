package utils

import (
	"math/rand"
	"reflect"
	"time"

	corev1 "k8s.io/api/core/v1"

	k8upv1 "github.com/k8up-io/k8up/v2/api/v1"
)

const _dataDirName = "k8up-dir"

func RandomStringGenerator(n int) string {
	var characters = []rune("abcdefghijklmnopqrstuvwxyz1234567890")
	rand.New(rand.NewSource(time.Now().UnixNano()))
	b := make([]rune, n)
	for i := range b {
		b[i] = characters[rand.Intn(len(characters))]
	}
	return string(b)
}

func ZeroLen(v interface{}) bool {
	return v == nil ||
		(reflect.ValueOf(v).Kind() == reflect.Ptr && reflect.ValueOf(v).IsNil()) ||
		(reflect.ValueOf(v).Kind() == reflect.Ptr && !reflect.ValueOf(v).IsNil() && reflect.ValueOf(v).Elem().Len() == 0)
}

func AppendTLSOptionsArgs(opts *k8upv1.TLSOptions, prefixArgName ...string) []string {
	var args []string
	if opts == nil {
		return args
	}

	var prefix string
	for _, v := range prefixArgName {
		prefix = v
	}

	caCertArg := "-caCert"
	clientCertArg := "-clientCert"
	clientKeyArg := "-clientKey"
	if prefix != "" {
		caCertArg = "-" + prefix + "CaCert"
		clientCertArg = "-" + prefix + "ClientCert"
		clientKeyArg = "-" + prefix + "ClientKey"
	}

	if opts.CACert != "" {
		args = append(args, []string{caCertArg, opts.CACert}...)
	}
	if opts.ClientCert != "" && opts.ClientKey != "" {
		addMoreArgs := []string{
			clientCertArg,
			opts.ClientCert,
			clientKeyArg,
			opts.ClientKey,
		}
		args = append(args, addMoreArgs...)
	}

	return args
}

func AttachTLSVolumes(volumes *[]k8upv1.RunnableVolumeSpec) []corev1.Volume {
	ku8pVolume := corev1.Volume{
		Name:         _dataDirName,
		VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}},
	}

	if volumes == nil {
		return []corev1.Volume{ku8pVolume}
	}

	moreVolumes := make([]corev1.Volume, 0, len(*volumes)+1)
	moreVolumes = append(moreVolumes, ku8pVolume)
	for _, v := range *volumes {
		vol := v

		var volumeSource corev1.VolumeSource
		if vol.PersistentVolumeClaim != nil {
			volumeSource.PersistentVolumeClaim = vol.PersistentVolumeClaim
		} else if vol.Secret != nil {
			volumeSource.Secret = vol.Secret
		} else if vol.ConfigMap != nil {
			volumeSource.ConfigMap = vol.ConfigMap
		} else {
			continue
		}

		addVolume := corev1.Volume{
			Name:         vol.Name,
			VolumeSource: volumeSource,
		}
		moreVolumes = append(moreVolumes, addVolume)
	}

	return moreVolumes
}

func AttachTLSVolumeMounts(k8upPodVarDir string, volumeMounts ...*[]corev1.VolumeMount) []corev1.VolumeMount {
	k8upVolumeMount := corev1.VolumeMount{
		Name:      _dataDirName,
		MountPath: k8upPodVarDir,
	}

	if len(volumeMounts) == 0 {
		return []corev1.VolumeMount{k8upVolumeMount}
	}

	var moreVolumeMounts []corev1.VolumeMount
	moreVolumeMounts = append(moreVolumeMounts, k8upVolumeMount)
	for _, vm := range volumeMounts {
		if vm == nil {
			continue
		}

		for _, v1 := range *vm {
			vm1 := v1
			var isExist bool

			for _, v2 := range moreVolumeMounts {
				vm2 := v2
				if vm1.Name == vm2.Name && vm1.MountPath == vm2.MountPath {
					isExist = true
					break
				}
			}

			if isExist {
				continue
			}

			moreVolumeMounts = append(moreVolumeMounts, vm1)
		}
	}

	return moreVolumeMounts
}
