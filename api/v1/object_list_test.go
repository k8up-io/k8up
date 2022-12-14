package v1_test

import (
	"testing"

	k8upv1 "github.com/k8up-io/k8up/v2/api/v1"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestGetObjectList(t *testing.T) {

	type jobObjectList interface {
		GetJobObjects() k8upv1.JobObjectList
	}

	testCases := map[string]struct {
		desc       string
		createList func(itemName1, itemName2 string) jobObjectList
	}{
		"Archive": {
			createList: func(itemName1, itemName2 string) jobObjectList {
				return &k8upv1.ArchiveList{
					Items: []k8upv1.Archive{
						{ObjectMeta: metav1.ObjectMeta{Name: itemName1}},
						{ObjectMeta: metav1.ObjectMeta{Name: itemName2}},
					},
				}
			},
		},
		"Backup": {
			createList: func(itemName1, itemName2 string) jobObjectList {
				return &k8upv1.BackupList{
					Items: []k8upv1.Backup{
						{ObjectMeta: metav1.ObjectMeta{Name: itemName1}},
						{ObjectMeta: metav1.ObjectMeta{Name: itemName2}},
					},
				}
			},
		},
		"Check": {
			createList: func(itemName1, itemName2 string) jobObjectList {
				return &k8upv1.CheckList{
					Items: []k8upv1.Check{
						{ObjectMeta: metav1.ObjectMeta{Name: itemName1}},
						{ObjectMeta: metav1.ObjectMeta{Name: itemName2}},
					},
				}
			},
		},
		"Prune": {
			createList: func(itemName1, itemName2 string) jobObjectList {
				return &k8upv1.PruneList{
					Items: []k8upv1.Prune{
						{ObjectMeta: metav1.ObjectMeta{Name: itemName1}},
						{ObjectMeta: metav1.ObjectMeta{Name: itemName2}},
					},
				}
			},
		},
		"Restore": {
			createList: func(itemName1, itemName2 string) jobObjectList {
				return &k8upv1.RestoreList{
					Items: []k8upv1.Restore{
						{ObjectMeta: metav1.ObjectMeta{Name: itemName1}},
						{ObjectMeta: metav1.ObjectMeta{Name: itemName2}},
					},
				}
			},
		},
	}
	for name, tC := range testCases {
		t.Run(name, func(t *testing.T) {
			name1 := "obj1"
			name2 := "obj2"
			list := tC.createList(name1, name2).GetJobObjects()
			assert.Equal(t, name1, list[0].GetName())
			assert.Equal(t, name2, list[1].GetName())
		})
	}
}
