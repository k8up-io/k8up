package v1alpha1_test

import (
	"testing"

	"github.com/vshn/k8up/api/v1alpha1"

	"github.com/stretchr/testify/assert"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestGetObjectList(t *testing.T) {

	type jobObjectList interface {
		GetJobObjects() v1alpha1.JobObjectList
	}

	testCases := map[string]struct {
		desc       string
		createList func(itemName1, itemName2 string) jobObjectList
	}{
		"Archive": {
			createList: func(itemName1, itemName2 string) jobObjectList {
				return &v1alpha1.ArchiveList{
					Items: []v1alpha1.Archive{
						{ObjectMeta: v1.ObjectMeta{Name: itemName1}},
						{ObjectMeta: v1.ObjectMeta{Name: itemName2}},
					},
				}
			},
		},
		"Backup": {
			createList: func(itemName1, itemName2 string) jobObjectList {
				return &v1alpha1.BackupList{
					Items: []v1alpha1.Backup{
						{ObjectMeta: v1.ObjectMeta{Name: itemName1}},
						{ObjectMeta: v1.ObjectMeta{Name: itemName2}},
					},
				}
			},
		},
		"Check": {
			createList: func(itemName1, itemName2 string) jobObjectList {
				return &v1alpha1.CheckList{
					Items: []v1alpha1.Check{
						{ObjectMeta: v1.ObjectMeta{Name: itemName1}},
						{ObjectMeta: v1.ObjectMeta{Name: itemName2}},
					},
				}
			},
		},
		"Prune": {
			createList: func(itemName1, itemName2 string) jobObjectList {
				return &v1alpha1.PruneList{
					Items: []v1alpha1.Prune{
						{ObjectMeta: v1.ObjectMeta{Name: itemName1}},
						{ObjectMeta: v1.ObjectMeta{Name: itemName2}},
					},
				}
			},
		},
		"Restore": {
			createList: func(itemName1, itemName2 string) jobObjectList {
				return &v1alpha1.RestoreList{
					Items: []v1alpha1.Restore{
						{ObjectMeta: v1.ObjectMeta{Name: itemName1}},
						{ObjectMeta: v1.ObjectMeta{Name: itemName2}},
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
			assert.Equal(t, name1, list[0].GetMetaObject().GetName())
			assert.Equal(t, name2, list[1].GetMetaObject().GetName())
		})
	}
}
