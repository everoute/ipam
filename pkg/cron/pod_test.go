package cron

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/everoute/ipam/api/ipam/v1alpha1"
)

func TestIsIPUsedByPod(t *testing.T) {
	t.Run("pending when pod status ip is allocated to the same pod", func(t *testing.T) {
		reader := newFakeReader(t, &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{Name: "pod1", Namespace: "ns1"},
			Status: corev1.PodStatus{
				Phase:  corev1.PodRunning,
				PodIP:  "10.10.65.2",
				PodIPs: []corev1.PodIP{{IP: "10.10.65.2"}},
			},
		})
		ippools := v1alpha1.IPPoolList{Items: []v1alpha1.IPPool{{
			Status: v1alpha1.IPPoolStatus{
				AllocatedIPs: map[string]v1alpha1.AllocateInfo{
					"10.10.65.2": {Type: v1alpha1.AllocateTypePod, ID: "ns1/pod1"},
				},
			},
		}}}
		c := &CleanStaleIP{k8sClient: newFakeClient(reader), k8sReader: reader}
		state, err := c.isIPUsedByPod(context.Background(), "10.10.65.1", types.NamespacedName{Namespace: "ns1", Name: "pod1"}, &ippools)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if state != podAllocationPending {
			t.Fatalf("expected pending, got %v", state)
		}
	})

	t.Run("used when pod status ip is not allocated in ippool", func(t *testing.T) {
		reader := newFakeReader(t, &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{Name: "pod1", Namespace: "ns1"},
			Status: corev1.PodStatus{
				Phase:  corev1.PodRunning,
				PodIP:  "10.10.65.2",
				PodIPs: []corev1.PodIP{{IP: "10.10.65.2"}},
			},
		})
		ippools := v1alpha1.IPPoolList{}
		c := &CleanStaleIP{k8sClient: newFakeClient(reader), k8sReader: reader}
		state, err := c.isIPUsedByPod(context.Background(), "10.10.65.1", types.NamespacedName{Namespace: "ns1", Name: "pod1"}, &ippools)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if state != podAllocationUsed {
			t.Fatalf("expected used, got %v", state)
		}
	})

	t.Run("used when pod status ip is allocated to another pod", func(t *testing.T) {
		reader := newFakeReader(t, &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{Name: "pod1", Namespace: "ns1"},
			Status: corev1.PodStatus{
				Phase:  corev1.PodRunning,
				PodIP:  "10.10.65.2",
				PodIPs: []corev1.PodIP{{IP: "10.10.65.2"}},
			},
		})
		ippools := v1alpha1.IPPoolList{Items: []v1alpha1.IPPool{{
			Status: v1alpha1.IPPoolStatus{
				AllocatedIPs: map[string]v1alpha1.AllocateInfo{
					"10.10.65.2": {Type: v1alpha1.AllocateTypePod, ID: "ns1/pod2"},
				},
			},
		}}}
		c := &CleanStaleIP{k8sClient: newFakeClient(reader), k8sReader: reader}
		state, err := c.isIPUsedByPod(context.Background(), "10.10.65.1", types.NamespacedName{Namespace: "ns1", Name: "pod1"}, &ippools)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if state != podAllocationUsed {
			t.Fatalf("expected used, got %v", state)
		}
	})

	t.Run("pending when one of pod status ips is allocated to the same pod", func(t *testing.T) {
		reader := newFakeReader(t, &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{Name: "pod1", Namespace: "ns1"},
			Status: corev1.PodStatus{
				Phase:  corev1.PodRunning,
				PodIP:  "10.10.65.2",
				PodIPs: []corev1.PodIP{{IP: "10.10.65.2"}, {IP: "10.10.65.3"}},
			},
		})
		ippools := v1alpha1.IPPoolList{Items: []v1alpha1.IPPool{{
			Status: v1alpha1.IPPoolStatus{
				AllocatedIPs: map[string]v1alpha1.AllocateInfo{
					"10.10.65.3": {Type: v1alpha1.AllocateTypePod, ID: "ns1/pod1"},
				},
			},
		}}}
		c := &CleanStaleIP{k8sClient: newFakeClient(reader), k8sReader: reader}
		state, err := c.isIPUsedByPod(context.Background(), "10.10.65.1", types.NamespacedName{Namespace: "ns1", Name: "pod1"}, &ippools)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if state != podAllocationPending {
			t.Fatalf("expected pending, got %v", state)
		}
	})

	t.Run("stale when pod is not found", func(t *testing.T) {
		reader := newFakeReader(t)
		ippools := v1alpha1.IPPoolList{}

		c := &CleanStaleIP{k8sClient: newFakeClient(reader), k8sReader: reader}
		state, err := c.isIPUsedByPod(context.Background(), "10.10.65.1", types.NamespacedName{Namespace: "ns1", Name: "pod1"}, &ippools)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if state != podAllocationStale {
			t.Fatalf("expected stale, got %v", state)
		}
	})
}

func newFakeReader(t *testing.T, objs ...client.Object) client.Reader {
	t.Helper()
	r := &staticReader{}
	for _, obj := range objs {
		pod, ok := obj.(*corev1.Pod)
		if !ok {
			t.Fatalf("unsupported object type %T", obj)
		}
		podCopy := pod.DeepCopy()
		r.pods = append(r.pods, podCopy)
	}
	return r
}

type staticReader struct {
	pods []*corev1.Pod
}

func (r *staticReader) Get(_ context.Context, key client.ObjectKey, obj client.Object, _ ...client.GetOption) error {
	pod, ok := obj.(*corev1.Pod)
	if !ok {
		return nil
	}
	for _, item := range r.pods {
		if item.Namespace == key.Namespace && item.Name == key.Name {
			*pod = *item.DeepCopy()
			return nil
		}
	}
	return apierrors.NewNotFound(schema.GroupResource{Group: "", Resource: "pods"}, key.Name)
}

func (r *staticReader) List(context.Context, client.ObjectList, ...client.ListOption) error {
	return nil
}

type fakeClient struct {
	reader client.Reader
}

func newFakeClient(reader client.Reader) client.Client {
	return &fakeClient{reader: reader}
}

func (f *fakeClient) Get(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
	return f.reader.Get(ctx, key, obj, opts...)
}

func (f *fakeClient) List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	return f.reader.List(ctx, list, opts...)
}

func (f *fakeClient) Create(context.Context, client.Object, ...client.CreateOption) error { return nil }
func (f *fakeClient) Delete(context.Context, client.Object, ...client.DeleteOption) error { return nil }
func (f *fakeClient) Update(context.Context, client.Object, ...client.UpdateOption) error { return nil }
func (f *fakeClient) Patch(context.Context, client.Object, client.Patch, ...client.PatchOption) error {
	return nil
}
func (f *fakeClient) DeleteAllOf(context.Context, client.Object, ...client.DeleteAllOfOption) error {
	return nil
}
func (f *fakeClient) Status() client.SubResourceWriter            { return fakeSubResourceClient{} }
func (f *fakeClient) SubResource(string) client.SubResourceClient { return fakeSubResourceClient{} }
func (f *fakeClient) Scheme() *runtime.Scheme                     { return runtime.NewScheme() }
func (f *fakeClient) RESTMapper() meta.RESTMapper                 { return nil }
func (f *fakeClient) GroupVersionKindFor(runtime.Object) (schema.GroupVersionKind, error) {
	return schema.GroupVersionKind{}, nil
}
func (f *fakeClient) IsObjectNamespaced(runtime.Object) (bool, error) { return true, nil }

type fakeSubResourceClient struct{}

func (fakeSubResourceClient) Create(context.Context, client.Object, client.Object, ...client.SubResourceCreateOption) error {
	return nil
}
func (fakeSubResourceClient) Update(context.Context, client.Object, ...client.SubResourceUpdateOption) error {
	return nil
}
func (fakeSubResourceClient) Patch(context.Context, client.Object, client.Patch, ...client.SubResourcePatchOption) error {
	return nil
}
func (fakeSubResourceClient) Get(context.Context, client.Object, client.Object, ...client.SubResourceGetOption) error {
	return nil
}
