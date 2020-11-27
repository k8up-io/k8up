// +build generate

package main

//go:generate go run sigs.k8s.io/controller-tools/cmd/controller-gen object:headerFile="hack/boilerplate.go.txt" paths="./..."
//go:generate go run sigs.k8s.io/controller-tools/cmd/controller-gen crd:trivialVersions=true rbac:roleName=manager-role webhook paths="./..." output:crd:artifacts:config=${CRD_ROOT_DIR}/v1beta1 crd:crdVersions=v1beta1
//go:generate go run sigs.k8s.io/controller-tools/cmd/controller-gen crd:trivialVersions=true rbac:roleName=manager-role webhook paths="./..." output:crd:artifacts:config=${CRD_ROOT_DIR}/v1      crd:crdVersions=v1
/// Run this file itself
//go:generate go run generate.go

import (
	"bufio"
	"fmt"
	"log"
	"os"
)

var patchFiles = []string{"v1beta1/backup.appuio.ch_prebackuppods.yaml"}

// controller-gen 0.3 creates CRDs with apiextensions.k8s.io/v1beta1, but some generated properties aren't valid for that version
// in K8s 1.18+. We would have to switch to apiextensions.k8s.io/v1, but that would make the CRD incompatible with OpenShift 3.11.
// So we have to patch the CRD in post-generation.
// See https://github.com/kubernetes/kubernetes/issues/91395
func main() {
	workdir, _ :=  os.Getwd()
	log.Println("Running post-generate in "+ workdir)
	for _, file := range patchFiles {
		fileName := os.Getenv("CRD_ROOT_DIR") + "/" + file
		log.Println(fmt.Sprintf("Reading file %s", fileName))
		lines, err := readLines(fileName)
		if err != nil {
			log.Fatalf("readLines: %s", err)
		}
		count := 0
		var result []string
		for i, line := range lines {
			if line == "                            - protocol" {
				count++
				log.Println(fmt.Sprintf("Removed 'protocol' in line %d", i))
			} else {
				result = append(result, line)
			}
		}

		log.Println(fmt.Sprintf("Writing new file to %s", fileName))
		if err := writeLines(result, fileName); err != nil {
			log.Fatalf("writeLines: %s", err)
		}
	}
}

// readLines reads a whole file into memory
// and returns a slice of its lines.
func readLines(path string) ([]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var lines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	return lines, scanner.Err()
}

// writeLines writes the lines to the given file.
func writeLines(lines []string, path string) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	w := bufio.NewWriter(file)
	for _, line := range lines {
		fmt.Fprintln(w, line)
	}
	return w.Flush()
}
