package cron

import (
	"context"
	"time"

	"k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type ProcessFun func(context.Context, client.Client)

type CleanStaleIP struct {
	period       time.Duration
	k8sClient    client.Client
	processFuncs []ProcessFun
}

func NewCleanStaleIP(period time.Duration, k8sClient client.Client) *CleanStaleIP {
	c := CleanStaleIP{
		period:       period,
		k8sClient:    k8sClient,
		processFuncs: make([]ProcessFun, 0),
	}
	c.RegistryCleanFunc(cleanStaleIPForPod)

	return &c
}

func (c *CleanStaleIP) Run(ctx context.Context) {
	go wait.NonSlidingUntilWithContext(ctx, c.process, c.period)
}

func (c *CleanStaleIP) RegistryCleanFunc(f ProcessFun) {
	if c.processFuncs == nil {
		c.processFuncs = make([]ProcessFun, 0)
	}
	c.processFuncs = append(c.processFuncs, f)
}

func (c *CleanStaleIP) process(ctx context.Context) {
	for i := range c.processFuncs {
		c.processFuncs[i](ctx, c.k8sClient)
	}
}
