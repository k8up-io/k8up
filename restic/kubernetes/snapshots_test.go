package kubernetes

import (
	"reflect"
	"testing"

	k8upv1 "github.com/k8up-io/k8up/v2/api/v1"
	"github.com/stretchr/testify/assert"
	"k8s.io/utils/ptr"
)

func Test_diff(t *testing.T) {
	type args struct {
		a        *k8upv1.SnapshotList
		b        *k8upv1.SnapshotList
		diffFunc func(snap k8upv1.Snapshot) error
	}
	tests := []struct {
		name      string
		args      args
		wantErr   bool
		diffCount int
	}{
		{
			name:      "GivenEmptyB_ThenExpectDiff",
			diffCount: 1,
			args: args{
				a: &k8upv1.SnapshotList{
					Items: []k8upv1.Snapshot{
						{
							Spec: k8upv1.SnapshotSpec{
								ID: ptr.To("2"),
							},
						},
					},
				},
				b: &k8upv1.SnapshotList{
					Items: []k8upv1.Snapshot{},
				},
			},
			wantErr: false,
		},
		{
			name:      "GivenIdentcal_ThenExpectNoDiff",
			diffCount: 0,
			args: args{
				a: &k8upv1.SnapshotList{
					Items: []k8upv1.Snapshot{
						{
							Spec: k8upv1.SnapshotSpec{
								ID: ptr.To("1"),
							},
						},
					},
				},
				b: &k8upv1.SnapshotList{
					Items: []k8upv1.Snapshot{
						{
							Spec: k8upv1.SnapshotSpec{
								ID: ptr.To("1"),
							},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name:      "GivenInAButNotB_ThenExpectDiff",
			diffCount: 2,
			args: args{
				a: &k8upv1.SnapshotList{
					Items: []k8upv1.Snapshot{
						{
							Spec: k8upv1.SnapshotSpec{
								ID: ptr.To("1"),
							},
						},
						{
							Spec: k8upv1.SnapshotSpec{
								ID: ptr.To("2"),
							},
						},
						{
							Spec: k8upv1.SnapshotSpec{
								ID: ptr.To("3"),
							},
						},
					},
				},
				b: &k8upv1.SnapshotList{
					Items: []k8upv1.Snapshot{
						{
							Spec: k8upv1.SnapshotSpec{
								ID: ptr.To("1"),
							},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name:      "GivenInBNotInA_ThenExpectNoDiff",
			diffCount: 0,
			args: args{
				a: &k8upv1.SnapshotList{
					Items: []k8upv1.Snapshot{},
				},
				b: &k8upv1.SnapshotList{
					Items: []k8upv1.Snapshot{
						{
							Spec: k8upv1.SnapshotSpec{
								ID: ptr.To("2"),
							},
						},
					},
				},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			counter := 0
			tt.args.diffFunc = func(snap k8upv1.Snapshot) error { counter++; return nil }

			if err := diff(tt.args.a, tt.args.b, tt.args.diffFunc); (err != nil) != tt.wantErr {
				t.Errorf("diff() error = %v, wantErr %v", err, tt.wantErr)
			}

			assert.Equal(t, tt.diffCount, counter, "The diff doesn't match")

		})
	}
}

func Test_filterByRepo(t *testing.T) {
	type args struct {
		list *k8upv1.SnapshotList
		repo string
	}
	tests := []struct {
		name string
		args args
		want *k8upv1.SnapshotList
	}{
		{
			name: "GivenMatchingRepos_ThenExpectSameList",
			want: &k8upv1.SnapshotList{
				Items: []k8upv1.Snapshot{
					{
						Spec: k8upv1.SnapshotSpec{
							Repository: ptr.To("myrepo"),
						},
					},
					{
						Spec: k8upv1.SnapshotSpec{
							Repository: ptr.To("myrepo"),
						},
					},
				},
			},
			args: args{
				repo: "myrepo",
				list: &k8upv1.SnapshotList{
					Items: []k8upv1.Snapshot{
						{
							Spec: k8upv1.SnapshotSpec{
								Repository: ptr.To("myrepo"),
							},
						},
						{
							Spec: k8upv1.SnapshotSpec{
								Repository: ptr.To("myrepo"),
							},
						},
					},
				},
			},
		},
		{
			name: "GivenDifferentRepos_ThenExpectDifferentList",
			want: &k8upv1.SnapshotList{
				Items: []k8upv1.Snapshot{
					{
						Spec: k8upv1.SnapshotSpec{
							Repository: ptr.To("myrepo"),
						},
					},
				},
			},
			args: args{
				repo: "myrepo",
				list: &k8upv1.SnapshotList{
					Items: []k8upv1.Snapshot{
						{
							Spec: k8upv1.SnapshotSpec{
								Repository: ptr.To("yourrepo"),
							},
						},
						{
							Spec: k8upv1.SnapshotSpec{
								Repository: ptr.To("myrepo"),
							},
						},
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := filterByRepo(tt.args.list, tt.args.repo); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("filterByRepo() = %v, want %v", got, tt.want)
			}
		})
	}
}
