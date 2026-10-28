package control

import (
	"math/bits"

	log "github.com/sirupsen/logrus"
)

// Control is to control the plugins.
type Control[CTX any] struct {
	Service string
	Cache   map[int64]uint8 // map[gid]isdisable
	Options Options[CTX]
	Manager *Manager[CTX]
}

// NewControl returns Manager with settings.
func (manager *Manager[CTX]) NewControl(service string, o *Options[CTX]) *Control[CTX] {
	var c GroupConfig
	m := &Control[CTX]{
		Service: service,
		Cache:   make(map[int64]uint8, 16),
		Options: func() Options[CTX] {
			if o == nil {
				return Options[CTX]{}
			}
			return *o
		}(),
		Manager: manager,
	}
	manager.Lock()
	defer manager.Unlock()
	manager.M[service] = m
	err := manager.D.Create(service, &c)
	if err != nil {
		panic(err)
	}
	err = manager.D.Create(service+"ban", &BanStatus{})
	if err != nil {
		panic(err)
	}
	err = manager.D.Find(m.Service, &c, "WHERE gid=0")
	if err == nil {
		if bits.RotateLeft64(uint64(c.Disable), 1)&1 == 1 {
			m.Options.DisableOnDefault = !m.Options.DisableOnDefault
		}
	}
	return m
}

// Enable enables a group to pass the Manager.
// groupID == 0 (ALL) will operate on all grps.
func (m *Control[CTX]) Enable(groupID int64) {
	var c GroupConfig
	m.Manager.RLock()
	err := m.Manager.D.Find(m.Service, &c, "WHERE gid = ?", groupID)
	m.Manager.RUnlock()
	if err != nil {
		c.GroupID = groupID
	}
	c.Disable = int64(uint64(c.Disable) & 0xffffffff_fffffffe)
	m.Manager.Lock()
	m.Cache[groupID] = 0
	err = m.Manager.D.Insert(m.Service, &c)
	m.Manager.Unlock()
	if err != nil {
		log.Errorf("[control] %v", err)
	}
}

// Disable disables a group to pass the Manager.
// groupID == 0 (ALL) will operate on all grps.
func (m *Control[CTX]) Disable(groupID int64) {
	var c GroupConfig
	m.Manager.RLock()
	err := m.Manager.D.Find(m.Service, &c, "WHERE gid = ?", groupID)
	m.Manager.RUnlock()
	if err != nil {
		c.GroupID = groupID
	}
	c.Disable |= 1
	m.Manager.Lock()
	m.Cache[groupID] = 1
	err = m.Manager.D.Insert(m.Service, &c)
	m.Manager.Unlock()
	if err != nil {
		log.Errorf("[control] %v", err)
	}
}

// Reset resets the default config of a group.
// groupID == 0 (ALL) is not allowed.
func (m *Control[CTX]) Reset(groupID int64) {
	if groupID != 0 {
		m.Manager.Lock()
		if m.Options.DisableOnDefault {
			m.Cache[groupID] = 1
		} else {
			m.Cache[groupID] = 0
		}
		err := m.Manager.D.Del(m.Service, "WHERE gid = ?", groupID)
		m.Manager.Unlock()
		if err != nil {
			log.Errorf("[control] %v", err)
		}
	}
}

// IsEnabledIn 查询开启群
// 当全局未配置或与默认相同时, 状态取决于单独配置, 后备为默认配置；
func (m *Control[CTX]) IsEnabledIn(gid int64) bool {
	var c GroupConfig
	var err error

	// 全局配置：0=启用, 1=禁用, 2=未配置
	var globalDisable uint8
	m.Manager.RLock()
	globalDisable, ok := m.Cache[0]
	m.Manager.RUnlock()
	if !ok {
		m.Manager.RLock()
		err = m.Manager.D.Find(m.Service, &c, "WHERE gid=0")
		m.Manager.RUnlock()
		m.Manager.Lock()
		if err == nil && c.GroupID == 0 {
			if c.Disable&1 == 0 {
				globalDisable = 0
			} else {
				globalDisable = 1
			}
		} else {
			globalDisable = 2
		}
		m.Cache[0] = globalDisable
		m.Manager.Unlock()
		log.Debugf("[control] cache plugin %s of all : %v", m.Service, globalDisable)
	}

	// 单独配置：0=启用, 1=禁用, 2=未配置
	var groupDisable uint8
	m.Manager.RLock()
	groupDisable, ok = m.Cache[gid]
	m.Manager.RUnlock()
	if !ok {
		m.Manager.RLock()
		err = m.Manager.D.Find(m.Service, &c, "WHERE gid = ?", gid)
		m.Manager.RUnlock()
		if err == nil && gid == c.GroupID {
			m.Manager.Lock()
			if c.Disable&1 == 0 {
				groupDisable = 0
			} else {
				groupDisable = 1
			}
			m.Cache[gid] = groupDisable
			m.Manager.Unlock()
			log.Debugf("[control] cache plugin %s of grp %d : %v", m.Service, gid, groupDisable)
		} else {
			// 未配置单独配置，用默认值
			m.Manager.Lock()
			if m.Options.DisableOnDefault {
				groupDisable = 1
			} else {
				groupDisable = 2
			}
			m.Cache[gid] = groupDisable
			m.Manager.Unlock()
			log.Debugf("[control] cache plugin %s of grp %d (default) : %v", m.Service, gid, groupDisable)
		}
	}

	switch globalDisable {
	case 0: // 如果全局启用单独配置是否为禁止(1)
		return groupDisable != 1
	case 1: // 如果全局禁止看单独配置是否为启用(0)
		return groupDisable == 0
	default:
		return globalDisable != 1 && groupDisable != 1
	}
}

// Handler 返回 预处理器
func (m *Control[CTX]) Handler(gid, uid int64) bool {
	if m.Manager.IsBlocked(uid) {
		return false
	}
	grp := gid
	if grp == 0 {
		// 个人用户
		grp = -uid
	}
	if !m.Manager.CanResponse(grp) || m.IsBannedIn(uid, grp) {
		return false
	}
	return m.IsEnabledIn(grp)
}

// String 打印帮助
func (m *Control[CTX]) String() string {
	return m.Options.Help
}
