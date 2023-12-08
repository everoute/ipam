package v1alpha1

import (
	"errors"
	"fmt"
	"net"
	"sync"
	"time"
)

type pool struct {
	IPPool
	expireTime time.Time
}
type cache struct {
	newAddPools []pool
	sync.Mutex
}

var mycache = cache{newAddPools: []pool{}}

func ValidatePool(poolList IPPoolList, wantAdd IPPool, old string) error {
	mycache.Lock()
	defer mycache.Unlock()
	wantName := wantAdd.GetNamespace() + `/` + wantAdd.GetName()
	noOld := false
	if old == "" {
		noOld = true
	}
	next := []pool{}
	defer func() {
		mycache.newAddPools = next
	}()
	now := time.Now()
	ippools := poolList.Items
	for i := 0; i < len(mycache.newAddPools); i++ {
		curPool := mycache.newAddPools[i]
		curPoolName := curPool.Namespace + `/` + curPool.Name
		if now.Before(curPool.expireTime) && (noOld || curPoolName != old) {
			next = append(next, curPool)
			ippools = append(ippools, curPool.IPPool)
		}
	}
	thisip, thisnet, err := net.ParseCIDR(wantAdd.Spec.CIDR)
	if err != nil {
		return errors.New("invalid cidr")
	}
	for i := 0; i < len(ippools); i++ {
		compareip, comparenet, _ := net.ParseCIDR(ippools[i].Spec.CIDR)
		curPoolName := ippools[i].Namespace + `/` + ippools[i].Name
		if noOld || curPoolName != old {
			if thisnet.Contains(compareip) || comparenet.Contains(thisip) {
				return fmt.Errorf("%s (want add) conflict with %s (exist). And(cidr) want is %s exist is %s",
					wantName, curPoolName, wantAdd.Spec.CIDR, ippools[i].Spec.CIDR)
			}
		}
	}
	next = append(next, pool{IPPool: wantAdd, expireTime: now.Add(5 * time.Second)})
	return nil
}
