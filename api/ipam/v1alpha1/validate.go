package v1alpha1

import (
	"fmt"
	"sync"
	"time"

	"github.com/everoute/ipam/pkg/utils"
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

	// delete ippool
	if wantAdd.Spec.CIDR == "" && wantAdd.Spec.Start == "" {
		return nil
	}

	thisStartIP := wantAdd.StartIP()
	thisEndIP := wantAdd.EndIP()
	for i := 0; i < len(ippools); i++ {
		curPoolName := ippools[i].Namespace + `/` + ippools[i].Name
		if noOld || curPoolName != old {
			if utils.IPBiggerThan(thisStartIP, ippools[i].EndIP()) {
				continue
			}
			if utils.IPBiggerThan(ippools[0].StartIP(), thisEndIP) {
				continue
			}
			return fmt.Errorf("%s (want add) conflict with %s (exist)", wantName, curPoolName)
		}
	}

	next = append(next, pool{IPPool: wantAdd, expireTime: now.Add(5 * time.Second)})
	return nil
}
