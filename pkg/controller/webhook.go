package controller

import (
	"bytes"
	"context"
	"fmt"

	admv1 "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"github.com/everoute/ipam/pkg/constants"
)

// WebhookReconciler watch webhook
type WebhookReconciler struct {
	client.Client
	Scheme    *runtime.Scheme
	Namespace string
}

// Reconcile receive webhook from work queue
func (r *WebhookReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	klog.Infof("WebhookReconciler received webhook %s reconcile", req.NamespacedName)

	secret := &corev1.Secret{}
	secretReq := types.NamespacedName{
		Name:      constants.ECPWebhookSecretName,
		Namespace: r.Namespace,
	}

	if err := r.Get(ctx, secretReq, secret); err != nil {
		klog.Fatalf("could not find secret %s/%s, err: %s", secretReq.Namespace, secretReq.Name, err)
	}

	webhook := &admv1.ValidatingWebhookConfiguration{}
	if err := r.Get(ctx, req.NamespacedName, webhook); err != nil {
		klog.Fatalf("could not find secret %s/%s, err: %s", secretReq.Namespace, secretReq.Name, err)
	}

	// update webhook
	webhookObj := &admv1.ValidatingWebhookConfiguration{}

	if err := r.Get(ctx, req.NamespacedName, webhookObj); err != nil {
		return ctrl.Result{}, err
	}
	if bytes.Equal(webhookObj.Webhooks[0].ClientConfig.CABundle, secret.Data["ca.crt"]) {
		return ctrl.Result{}, nil
	}
	webhookObj.Webhooks[0].ClientConfig.CABundle = append([]byte{}, secret.Data["ca.crt"]...)
	return ctrl.Result{}, r.Update(ctx, webhookObj)
}

// SetupWithManager create and add Webhook Controller to the manager.
func (r *WebhookReconciler) SetupWithManager(mgr ctrl.Manager) error {
	if mgr == nil {
		return fmt.Errorf("can't setup with nil manager")
	}

	c, err := controller.New("webhook-controller", mgr, controller.Options{
		MaxConcurrentReconciles: 1,
		Reconciler:              r,
	})
	if err != nil {
		return err
	}

	err = c.Watch(source.Kind(mgr.GetCache(), &admv1.ValidatingWebhookConfiguration{}), &handler.Funcs{
		CreateFunc: func(_ context.Context, e event.CreateEvent, q workqueue.RateLimitingInterface) {
			if e.Object == nil {
				klog.Errorf("receive create event with no object %v", e)
				return
			}
			if e.Object.GetName() == constants.ValidatingWebhookConfigurationName {
				q.Add(ctrl.Request{NamespacedName: types.NamespacedName{
					Name: constants.ValidatingWebhookConfigurationName,
				}})
			}
		},
		UpdateFunc: func(_ context.Context, e event.UpdateEvent, q workqueue.RateLimitingInterface) {
			newWebhook := e.ObjectNew.(*admv1.ValidatingWebhookConfiguration)
			if newWebhook.ObjectMeta.GetName() == constants.ValidatingWebhookConfigurationName {
				q.Add(ctrl.Request{NamespacedName: types.NamespacedName{
					Name: constants.ValidatingWebhookConfigurationName,
				}})
			}
		},
	})

	return err
}
