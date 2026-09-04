package controllers

import (
	"context"
	"testing"

	expositionv1 "github.com/cloudogu/k8s-exposition-lib/api/v1"
	"github.com/cloudogu/k8s-registry-lib/repository"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	k8stypes "k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func TestMaintenanceConfigMapPredicate(t *testing.T) {
	maintCM := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name: repository.MaintenanceConfigMapName, Namespace: testNamespace, ResourceVersion: "1",
		},
		Data: map[string]string{"active": "false"},
	}
	maintCMToggled := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name: repository.MaintenanceConfigMapName, Namespace: testNamespace, ResourceVersion: "2",
		},
		Data: map[string]string{"active": "true"},
	}
	otherCM := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "other", Namespace: testNamespace, ResourceVersion: "1"},
		Data:       map[string]string{"active": "true"},
	}
	otherCMToggled := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "other", Namespace: testNamespace, ResourceVersion: "2"},
		Data:       map[string]string{"active": "false"},
	}

	p := maintenanceConfigMapPredicate()

	t.Run("Create accepts maintenance ConfigMap", func(t *testing.T) {
		assert.True(t, p.Create(event.CreateEvent{Object: maintCM}))
	})
	t.Run("Create rejects other ConfigMap", func(t *testing.T) {
		assert.False(t, p.Create(event.CreateEvent{Object: otherCM}))
	})
	t.Run("Update accepts maintenance ConfigMap when the active flag is toggled", func(t *testing.T) {
		assert.True(t, p.Update(event.UpdateEvent{ObjectOld: maintCM, ObjectNew: maintCMToggled}))
	})
	t.Run("Update accepts maintenance ConfigMap when the flag is toggled back", func(t *testing.T) {
		assert.True(t, p.Update(event.UpdateEvent{ObjectOld: maintCMToggled, ObjectNew: maintCM}))
	})
	// The informer resync re-delivers updates with an unchanged resource version.
	t.Run("Update rejects maintenance ConfigMap without resource version change", func(t *testing.T) {
		assert.False(t, p.Update(event.UpdateEvent{ObjectOld: maintCM, ObjectNew: maintCM}))
	})
	t.Run("Update rejects non-maintenance ConfigMap", func(t *testing.T) {
		assert.False(t, p.Update(event.UpdateEvent{ObjectOld: otherCM, ObjectNew: otherCMToggled}))
	})
	t.Run("Delete accepts maintenance ConfigMap", func(t *testing.T) {
		assert.True(t, p.Delete(event.DeleteEvent{Object: maintCM}))
	})
	t.Run("Delete rejects other ConfigMap", func(t *testing.T) {
		assert.False(t, p.Delete(event.DeleteEvent{Object: otherCM}))
	})
	t.Run("Generic accepts maintenance ConfigMap", func(t *testing.T) {
		assert.True(t, p.Generic(event.GenericEvent{Object: maintCM}))
	})
	t.Run("Generic rejects other ConfigMap", func(t *testing.T) {
		assert.False(t, p.Generic(event.GenericEvent{Object: otherCM}))
	})
}

func TestMapRequestsFromMaintenanceConfigMap(t *testing.T) {
	doguLabelReq, err := labels.NewRequirement("dogu.name", selection.Exists, nil)
	require.NoError(t, err)
	doguLabelSelector := labels.NewSelector().Add(*doguLabelReq)
	expectedListOpts := &client.ListOptions{Namespace: testNamespace, LabelSelector: doguLabelSelector}

	maintCM := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{
		Name: repository.MaintenanceConfigMapName, Namespace: testNamespace,
	}}

	tests := []struct {
		name     string
		clientFn func(t *testing.T) client.Client
		newList  func() client.ObjectList
		logName  string
		want     []reconcile.Request
	}{
		{
			name: "list fails returns nil",
			clientFn: func(t *testing.T) client.Client {
				m := newMockK8sClient(t)
				m.EXPECT().
					List(t.Context(), &corev1.ServiceList{}, expectedListOpts).
					Return(assert.AnError)
				return m
			},
			newList: func() client.ObjectList { return &corev1.ServiceList{} },
			logName: "service-maintenance",
			want:    nil,
		},
		{
			name: "list empty returns empty slice",
			clientFn: func(t *testing.T) client.Client {
				m := newMockK8sClient(t)
				m.EXPECT().
					List(t.Context(), &corev1.ServiceList{}, expectedListOpts).
					Return(nil)
				return m
			},
			newList: func() client.ObjectList { return &corev1.ServiceList{} },
			logName: "service-maintenance",
			want:    []reconcile.Request{},
		},
		{
			name: "three Services produce three requests keyed by metadata.name",
			clientFn: func(t *testing.T) client.Client {
				m := newMockK8sClient(t)
				m.EXPECT().
					List(t.Context(), &corev1.ServiceList{}, expectedListOpts).
					Run(func(_ context.Context, list client.ObjectList, _ ...client.ListOption) {
						list.(*corev1.ServiceList).Items = []corev1.Service{
							{ObjectMeta: metav1.ObjectMeta{Name: "ldap", Namespace: testNamespace}},
							{ObjectMeta: metav1.ObjectMeta{Name: "scm", Namespace: testNamespace}},
							{ObjectMeta: metav1.ObjectMeta{Name: "redmine", Namespace: testNamespace}},
						}
					}).
					Return(nil)
				return m
			},
			newList: func() client.ObjectList { return &corev1.ServiceList{} },
			logName: "service-maintenance",
			want: []reconcile.Request{
				{NamespacedName: k8stypes.NamespacedName{Namespace: testNamespace, Name: "ldap"}},
				{NamespacedName: k8stypes.NamespacedName{Namespace: testNamespace, Name: "scm"}},
				{NamespacedName: k8stypes.NamespacedName{Namespace: testNamespace, Name: "redmine"}},
			},
		},
		{
			name: "three Expositions produce three requests keyed by metadata.name",
			clientFn: func(t *testing.T) client.Client {
				m := newMockK8sClient(t)
				m.EXPECT().
					List(t.Context(), &expositionv1.ExpositionList{}, expectedListOpts).
					Run(func(_ context.Context, list client.ObjectList, _ ...client.ListOption) {
						list.(*expositionv1.ExpositionList).Items = []expositionv1.Exposition{
							{ObjectMeta: metav1.ObjectMeta{Name: "ldap", Namespace: testNamespace}},
							{ObjectMeta: metav1.ObjectMeta{Name: "scm", Namespace: testNamespace}},
							{ObjectMeta: metav1.ObjectMeta{Name: "redmine", Namespace: testNamespace}},
						}
					}).
					Return(nil)
				return m
			},
			newList: func() client.ObjectList { return &expositionv1.ExpositionList{} },
			logName: "exposition-maintenance",
			want: []reconcile.Request{
				{NamespacedName: k8stypes.NamespacedName{Namespace: testNamespace, Name: "ldap"}},
				{NamespacedName: k8stypes.NamespacedName{Namespace: testNamespace, Name: "scm"}},
				{NamespacedName: k8stypes.NamespacedName{Namespace: testNamespace, Name: "redmine"}},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fn := mapRequestsFromMaintenanceConfigMap(tt.clientFn(t), tt.newList, tt.logName)
			got := fn(t.Context(), maintCM)
			assert.Equal(t, tt.want, got)
		})
	}
}
