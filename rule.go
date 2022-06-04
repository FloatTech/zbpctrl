// Package control 控制插件的启用与优先级等
package control

import (
	"crypto/md5"
	"encoding/binary"
	"fmt"
	"math/bits"
	"strconv"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/wdvxdr1123/ZeroBot/extension/ttl"
	"github.com/wdvxdr1123/ZeroBot/utils/helper"

	sql "github.com/FloatTech/sqlite"
)

type Manager[CTX any] struct {
	sync.RWMutex
	M map[string]*Control[CTX]
	D sql.Sqlite
	B *ttl.Cache[uintptr, bool]
}

func NewManager[CTX any](dbpath string, banmapttl time.Duration) Manager[CTX] {
	return Manager[CTX]{
		D: sql.Sqlite{DBPath: dbpath},
		B: ttl.NewCache[uintptr, bool](banmapttl),
	}
}

// Control is to control the plugins.
type Control[CTX any] struct {
	Service string
	Cache   map[int64]bool
	Options Options[CTX]
	Manager *Manager[CTX]
}

// newctrl returns Manager with settings.
func (manager *Manager[CTX]) NewControl(service string, o *Options[CTX]) *Control[CTX] {
	var c GroupConfig
	m := &Control[CTX]{
		Service: service,
		Cache:   make(map[int64]bool, 16),
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
	err := m.Manager.D.Find(m.Service, &c, "WHERE gid="+strconv.FormatInt(groupID, 10))
	m.Manager.RUnlock()
	if err != nil {
		c.GroupID = groupID
	}
	c.Disable = int64(uint64(c.Disable) & 0xffffffff_fffffffe)
	m.Manager.Lock()
	m.Cache[groupID] = true
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
	err := m.Manager.D.Find(m.Service, &c, "WHERE gid="+strconv.FormatInt(groupID, 10))
	m.Manager.RUnlock()
	if err != nil {
		c.GroupID = groupID
	}
	c.Disable |= 1
	m.Manager.Lock()
	m.Cache[groupID] = false
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
		delete(m.Cache, groupID)
		err := m.Manager.D.Del(m.Service, "WHERE gid="+strconv.FormatInt(groupID, 10))
		m.Manager.Unlock()
		if err != nil {
			log.Errorf("[control] %v", err)
		}
	}
}

// IsEnabledIn 查询开启群
// 当全局未配置或与默认相同时, 状态取决于单独配置, 后备为默认配置；
// 当全局与默认不同时, 状态取决于全局配置, 单独配置失效。
func (m *Control[CTX]) IsEnabledIn(gid int64) bool {
	var c GroupConfig
	var err error

	m.Manager.RLock()
	yes, ok := m.Cache[0]
	m.Manager.RUnlock()
	if !ok {
		m.Manager.RLock()
		err = m.Manager.D.Find(m.Service, &c, "WHERE gid=0")
		m.Manager.RUnlock()
		if err == nil && c.GroupID == 0 {
			log.Debugf("[control] plugin %s of all : %d", m.Service, c.Disable&1)
			yes = c.Disable&1 == 0
			ok = true
			m.Manager.Lock()
			m.Cache[0] = yes
			m.Manager.Unlock()
			log.Debugf("[control] cache plugin %s of grp %d : %v", m.Service, gid, yes)
		}
	}

	if ok && yes == m.Options.DisableOnDefault { // global enable status is different from default value
		return yes
	}

	m.Manager.RLock()
	yes, ok = m.Cache[gid]
	m.Manager.RUnlock()
	if ok {
		log.Debugf("[control] read cached %s of grp %d : %v", m.Service, gid, yes)
	} else {
		m.Manager.RLock()
		err = m.Manager.D.Find(m.Service, &c, "WHERE gid="+strconv.FormatInt(gid, 10))
		m.Manager.RUnlock()
		if err == nil && gid == c.GroupID {
			log.Debugf("[control] plugin %s of grp %d : %d", m.Service, c.GroupID, c.Disable&1)
			yes = c.Disable&1 == 0
			ok = true
			m.Manager.Lock()
			m.Cache[gid] = yes
			m.Manager.Unlock()
			log.Debugf("[control] cache plugin %s of grp %d : %v", m.Service, gid, yes)
		}
	}

	if ok {
		return yes
	}
	return !m.Options.DisableOnDefault
}

// Ban 禁止某人在某群使用本插件
func (m *Control[CTX]) Ban(uid, gid int64) {
	var err error
	var digest [16]byte
	if gid != 0 { // 特定群
		digest = md5.Sum(helper.StringToBytes(fmt.Sprintf("%d_%d", uid, gid)))
		m.Manager.RLock()
		err = m.Manager.D.Insert(m.Service+"ban", &BanStatus{ID: int64(binary.LittleEndian.Uint64(digest[:8])), UserID: uid, GroupID: gid})
		m.Manager.RUnlock()
		if err == nil {
			log.Debugf("[control] plugin %s is banned in grp %d for usr %d.", m.Service, gid, uid)
			return
		}
	}
	// 所有群
	digest = md5.Sum(helper.StringToBytes(fmt.Sprintf("%d_all", uid)))
	m.Manager.RLock()
	err = m.Manager.D.Insert(m.Service+"ban", &BanStatus{ID: int64(binary.LittleEndian.Uint64(digest[:8])), UserID: uid, GroupID: 0})
	m.Manager.RUnlock()
	if err == nil {
		log.Debugf("[control] plugin %s is banned in all grp for usr %d.", m.Service, uid)
	}
}

// Permit 允许某人在某群使用本插件
func (m *Control[CTX]) Permit(uid, gid int64) {
	var digest [16]byte
	if gid != 0 { // 特定群
		digest = md5.Sum(helper.StringToBytes(fmt.Sprintf("%d_%d", uid, gid)))
		m.Manager.RLock()
		_ = m.Manager.D.Del(m.Service+"ban", "WHERE id = "+strconv.FormatInt(int64(binary.LittleEndian.Uint64(digest[:8])), 10))
		m.Manager.RUnlock()
		log.Debugf("[control] plugin %s is permitted in grp %d for usr %d.", m.Service, gid, uid)
		return
	}
	// 所有群
	digest = md5.Sum(helper.StringToBytes(fmt.Sprintf("%d_all", uid)))
	m.Manager.RLock()
	_ = m.Manager.D.Del(m.Service+"ban", "WHERE id = "+strconv.FormatInt(int64(binary.LittleEndian.Uint64(digest[:8])), 10))
	m.Manager.RUnlock()
	log.Debugf("[control] plugin %s is permitted in all grp for usr %d.", m.Service, uid)
}

// IsBannedIn 某人是否在某群被 ban
func (m *Control[CTX]) IsBannedIn(uid, gid int64) bool {
	var b BanStatus
	var err error
	var digest [16]byte
	if gid != 0 {
		digest = md5.Sum(helper.StringToBytes(fmt.Sprintf("%d_%d", uid, gid)))
		m.Manager.RLock()
		err = m.Manager.D.Find(m.Service+"ban", &b, "WHERE id = "+strconv.FormatInt(int64(binary.LittleEndian.Uint64(digest[:8])), 10))
		m.Manager.RUnlock()
		if err == nil && gid == b.GroupID && uid == b.UserID {
			log.Debugf("[control] plugin %s is banned in grp %d for usr %d.", m.Service, b.GroupID, b.UserID)
			return true
		}
	}
	digest = md5.Sum(helper.StringToBytes(fmt.Sprintf("%d_all", uid)))
	m.Manager.RLock()
	err = m.Manager.D.Find(m.Service+"ban", &b, "WHERE id = "+strconv.FormatInt(int64(binary.LittleEndian.Uint64(digest[:8])), 10))
	m.Manager.RUnlock()
	if err == nil && b.GroupID == 0 && uid == b.UserID {
		log.Debugf("[control] plugin %s is banned in all grp for usr %d.", m.Service, b.UserID)
		return true
	}
	return false
}

// GetData 获取某个群的 62 位配置信息
func (m *Control[CTX]) GetData(gid int64) int64 {
	var c GroupConfig
	var err error
	m.Manager.RLock()
	err = m.Manager.D.Find(m.Service, &c, "WHERE gid="+strconv.FormatInt(gid, 10))
	m.Manager.RUnlock()
	if err == nil && gid == c.GroupID {
		log.Debugf("[control] plugin %s of grp %d : 0x%x", m.Service, c.GroupID, c.Disable>>1)
		return (c.Disable >> 1) & 0x3fffffff_ffffffff
	}
	return 0
}

// SetData 为某个群设置低 62 位配置数据
func (m *Control[CTX]) SetData(groupID int64, data int64) error {
	var c GroupConfig
	m.Manager.RLock()
	err := m.Manager.D.Find(m.Service, &c, "WHERE gid="+strconv.FormatInt(groupID, 10))
	m.Manager.RUnlock()
	if err != nil {
		c.GroupID = groupID
		if m.Options.DisableOnDefault {
			c.Disable = 1
		}
	}
	x := bits.RotateLeft64(uint64(c.Disable), 1)
	x &= 0x03
	x |= uint64(data) << 2
	c.Disable = int64(bits.RotateLeft64(x, -1))
	log.Debugf("[control] set plugin %s of grp %d : 0x%x", m.Service, c.GroupID, data)
	m.Manager.Lock()
	err = m.Manager.D.Insert(m.Service, &c)
	m.Manager.Unlock()
	if err != nil {
		log.Errorf("[control] %v", err)
	}
	return err
}

// Flip 改变全局默认启用状态
func (m *Control[CTX]) Flip() error {
	var c GroupConfig
	m.Manager.Lock()
	defer m.Manager.Unlock()
	m.Options.DisableOnDefault = !m.Options.DisableOnDefault
	err := m.Manager.D.Find(m.Service, &c, "WHERE gid=0")
	if err != nil && m.Options.DisableOnDefault {
		c.Disable = 1
	}
	x := bits.RotateLeft64(uint64(c.Disable), 1) &^ 1
	c.Disable = int64(bits.RotateLeft64(x, -1))
	log.Debugf("[control] flip plugin %s of all : %d", m.Service, c.GroupID, x&1)
	err = m.Manager.D.Insert(m.Service, &c)
	if err != nil {
		log.Errorf("[control] %v", err)
	}
	return err
}

// Handler 返回 预处理器
func (m *Control[CTX]) Handler(ctx uintptr, gid, uid int64) bool {
	grp := gid
	if grp == 0 {
		// 个人用户
		grp = -uid
	}
	ok := m.Manager.B.Get(ctx)
	if ok {
		return m.IsEnabledIn(grp)
	}
	isnotbanned := !m.IsBannedIn(uid, grp)
	if isnotbanned {
		m.Manager.B.Set(ctx, true)
		return m.IsEnabledIn(grp)
	}
	return false
}

// String 打印帮助
func (m *Control[CTX]) String() string {
	return m.Options.Help
}

// EnableMark 启用：●，禁用：○
type EnableMark bool

// String 打印启用状态
func (em EnableMark) String() string {
	if bool(em) {
		return "●"
	}
	return "○"
}

// EnableMarkIn 打印 ● 或 ○
func (m *Control[CTX]) EnableMarkIn(grp int64) EnableMark {
	return EnableMark(m.IsEnabledIn(grp))
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
	m := copyMap(manager.M)
	manager.RUnlock()
	for k, v := range m {
		if !iterator(k, v) {
			return
		}
	}
}

func copyMap[CTX any](m map[string]*Control[CTX]) map[string]*Control[CTX] {
	ret := make(map[string]*Control[CTX], len(m))
	for k, v := range m {
		ret[k] = v
	}
	return ret
}
