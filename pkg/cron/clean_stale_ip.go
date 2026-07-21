package cron

import (
	"context"
	"time"

	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/everoute/ipam/api/ipam/v1alpha1"
)

const (
	podCleanQueueKey         = "stale-pod-ip-cleaner"
	statefulSetCleanQueueKey = "stale-statefulset-ip-cleaner"
)

type podPendingKey struct {
	Pool types.NamespacedName
	IP   string
	Pod  types.NamespacedName
}

type podPendingInfo struct {
	AllocateInfo v1alpha1.AllocateInfo
}

type CleanStaleIP struct {
	period          time.Duration
	fastRetryPeriod time.Duration
	k8sReader       client.Reader
	k8sClient       client.Client
	queue           workqueue.DelayingInterface
	podPending      map[podPendingKey]podPendingInfo
}

func NewCleanStaleIP(period, fastRetryPeriod time.Duration, k8sClient client.Client, k8sReader client.Reader) *CleanStaleIP {
	if fastRetryPeriod <= 0 || fastRetryPeriod > period {
		fastRetryPeriod = period
	}
	return &CleanStaleIP{
		period:          period,
		fastRetryPeriod: fastRetryPeriod,
		k8sReader:       k8sReader,
		k8sClient:       k8sClient,
		queue:           workqueue.NewNamedDelayingQueue("stale-ip"),
		podPending:      make(map[podPendingKey]podPendingInfo),
	}
}

func (c *CleanStaleIP) Run(ctx context.Context) {
	go func() {
		<-ctx.Done()
		c.queue.ShutDown()
	}()

	c.queue.Add(podCleanQueueKey)
	c.queue.Add(statefulSetCleanQueueKey)

	go func() {
		for {
			if !c.processNextItem(ctx) {
				return
			}
		}
	}()
}

func (c *CleanStaleIP) processNextItem(ctx context.Context) bool {
	item, shutdown := c.queue.Get()
	if shutdown {
		return false
	}
	defer c.queue.Done(item)

	key, ok := item.(string)
	if !ok {
		return true
	}

	switch key {
	case podCleanQueueKey:
		nextDelay := c.period
		hasPending, err := c.cleanStaleIPForPod(ctx)
		if err != nil || hasPending {
			nextDelay = c.fastRetryPeriod
		}
		c.queue.AddAfter(podCleanQueueKey, nextDelay)
	case statefulSetCleanQueueKey:
		// Keep the original periodic cleanup flow for stale statefulset ips without fast retry.
		cleanStaleIPForStatefulSet(ctx, c.k8sClient, c.k8sReader)
		c.queue.AddAfter(statefulSetCleanQueueKey, c.period)
	}
	return true
}
