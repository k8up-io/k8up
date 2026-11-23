package executor

import (
	"testing"

	corev1 "k8s.io/api/core/v1"

	"github.com/k8up-io/k8up/v2/operator/cfg"
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

func TestDefaultEnv_PromFormat(t *testing.T) {
	defer func(format string) { cfg.Config.PromFormat = format }(cfg.Config.PromFormat)

	cfg.Config.PromFormat = ""
	if _, ok := DefaultEnv("ns").Vars["PROM_FORMAT"]; ok {
		t.Error("PROM_FORMAT should not be set without a configured format.")
	}

	cfg.Config.PromFormat = "text"
	v, ok := DefaultEnv("ns").Vars["PROM_FORMAT"]
	if !ok || *v.stringEnv != "text" {
		t.Error("PROM_FORMAT should be set to the configured format.")
	}
}
