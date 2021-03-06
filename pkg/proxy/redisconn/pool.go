// Copyright 2015 Reborndb Org. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package redisconn

import (
	"sync"
	"time"

	"github.com/juju/errors"
	"github.com/ngaut/pools"
)

const PoolIdleTimeoutSecond = 120

type CreateConnFunc func(addr string) (*Conn, error)

type Pool struct {
	p *pools.ResourcePool
}

func NewPool(addr string, capability int, f CreateConnFunc) *Pool {
	poolFunc := func() (pools.Resource, error) {
		r, err := f(addr)
		if err == nil && r == nil {
			return nil, errors.Errorf("cannot create nil connection")
		}
		return r, errors.Trace(err)
	}

	p := new(Pool)
	p.p = pools.NewResourcePool(poolFunc, capability, capability, PoolIdleTimeoutSecond*time.Second)
	return p
}

func (p *Pool) GetConn() (*Conn, error) {
	conn, err := p.p.Get()
	if err != nil {
		return nil, errors.Trace(err)
	} else {
		return conn.(*Conn), nil
	}
}

func (p *Pool) PutConn(c *Conn) {
	if c == nil {
		return
	} else if c.closed {
		// if c is closed, we will put nil
		p.p.Put(nil)
	} else {
		p.p.Put(c)
	}
}

func (p *Pool) Close() {
	p.p.Close()
}

type Pools struct {
	m sync.Mutex

	capability int

	mpools map[string]*Pool

	f CreateConnFunc
}

func NewPools(capability int, f CreateConnFunc) *Pools {
	p := new(Pools)
	p.f = f
	p.capability = capability
	p.mpools = make(map[string]*Pool)
	return p
}

func (p *Pools) GetConn(addr string) (*Conn, error) {
	p.m.Lock()
	pool, ok := p.mpools[addr]
	if !ok {
		pool = NewPool(addr, p.capability, p.f)
		p.mpools[addr] = pool
	}
	p.m.Unlock()

	return pool.GetConn()
}

func (p *Pools) PutConn(c *Conn) {
	if c == nil {
		return
	}

	p.m.Lock()
	pool, ok := p.mpools[c.addr]
	p.m.Unlock()
	if !ok {
		c.Close()
	} else {
		pool.PutConn(c)
	}
}

func (p *Pools) Close() {
	p.m.Lock()
	defer p.m.Unlock()

	for _, pool := range p.mpools {
		pool.Close()
	}

	p.mpools = map[string]*Pool{}
}
