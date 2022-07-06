// Package control 控制插件的启用与优先级等
package control

import (
	"os"
	"strings"
	"sync"
	"time"

	"github.com/wdvxdr1123/ZeroBot/extension/ttl"

	sql "github.com/FloatTech/sqlite"
)

type Manager[CTX any] struct {
	sync.RWMutex
	M map[string]*Control[CTX]
	D sql.Sqlite
	B *ttl.Cache[uintptr, bool]
}

func NewManager[CTX any](dbpath string, banmapttl time.Duration) (m Manager[CTX]) {
	switch {
	case dbpath == "":
		dbpath = "ctrl.db"
	case strings.HasSuffix(dbpath, "/"):
		err := os.MkdirAll(dbpath, 0755)
		if err != nil {
			panic(err)
		}
		dbpath += "ctrl.db"
	default:
		i := strings.LastIndex(dbpath, "/")
		if i > 0 {
			err := os.MkdirAll(dbpath[:i], 0755)
			if err != nil {
				panic(err)
			}
		}
	}
	m = Manager[CTX]{
		M: map[string]*Control[CTX]{},
		D: sql.Sqlite{DBPath: dbpath},
		B: ttl.NewCache[uintptr, bool](banmapttl),
	}
	err := m.D.Open(time.Hour * 24)
	if err != nil {
		panic(err)
	}
	err = m.initBlock()
	if err != nil {
		panic(err)
	}
	err = m.initResponse()
	if err != nil {
		panic(err)
	}
	return
}

// Lookup returns a Manager by the service name, if
// not exist, it will return nil.
func (manager *Manager[CTX]) Lookup(service string) (*Control[CTX], bool) {
	manager.RLock()
	m, ok := manager.M[service]
	manager.RUnlock()
	return m, ok
}

// ForEach iterates through managers.
func (manager *Manager[CTX]) ForEach(iterator func(key string, manager *Control[CTX]) bool) {
	manager.RLock()
	m := cpmp(manager.M)
	manager.RUnlock()
	for k, v := range m {
		if !iterator(k, v) {
			return
		}
	}
}

func cpmp[CTX any](m map[string]*Control[CTX]) map[string]*Control[CTX] {
	ret := make(map[string]*Control[CTX], len(m))
	for k, v := range m {
		ret[k] = v
	}
	return ret
}
