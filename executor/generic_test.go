package executor

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
)

func TestEnvVarConverter_Merge(t *testing.T) {
	vars := NewEnvVarConverter()
	vars.SetString("nooverridestr", "original")
	vars.SetEnvVarSource("nooverridesrc", &corev1.EnvVarSource{SecretKeyRef: &corev1.SecretKeySelector{Key: "original"}})
	vars.SetString("nomergestr", "original")
	vars.SetEnvVarSource("nomergesource", &corev1.EnvVarSource{SecretKeyRef: &corev1.SecretKeySelector{Key: "original"}})

	src := NewEnvVarConverter()
	src.SetString("nooverridestr", "updated")
	src.SetEnvVarSource("nooverridestr", &corev1.EnvVarSource{SecretKeyRef: &corev1.SecretKeySelector{Key: "updated"}})
	src.SetEnvVarSource("nooverridesrc", &corev1.EnvVarSource{SecretKeyRef: &corev1.SecretKeySelector{Key: "updated"}})
	src.SetString("newstr", "original")
	src.SetEnvVarSource("newsource", &corev1.EnvVarSource{SecretKeyRef: &corev1.SecretKeySelector{Key: "original"}})

	if err := vars.Merge(src); err != nil {
		t.Errorf("unable to merge: %v", err)
	}

	v := vars.Vars

	if *v["nooverridestr"].stringEnv != "original" {
		t.Error("nooverridestr should not have been updated.")
	}
	if v["nooverridestr"].envVarSource != nil {
		t.Error("nooverridestr should not have been updated.")
	}
	if v["nooverridesrc"].envVarSource.SecretKeyRef.Key != "original" {
		t.Error("nooverridesrc should not have been updated.")
	}

	if *v["nomergestr"].stringEnv != "original" {
		t.Error("nomergestr should not have been updated.")
	}
	if v["nomergesource"].envVarSource.SecretKeyRef.Key != "original" {
		t.Error("nomergesource should not have been updated.")
	}

	if *v["newstr"].stringEnv != "original" {
		t.Error("newstr should have been merged in.")
	}
	if v["newsource"].envVarSource.SecretKeyRef.Key != "original" {
		t.Error("nomergesource should have been merged in.")
	}
}
