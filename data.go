package control

import (
	"encoding/json"
	"errors"
	"math/bits"
	"strconv"

	log "github.com/sirupsen/logrus"
	"github.com/wdvxdr1123/ZeroBot/utils/helper"
)

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

// SetData 为某个群设置中间 62 位配置数据 (除高低位)
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

func (manager *Manager[CTX]) GetExtra(gid int64, obj any) error {
	if !manager.CanResponse(gid) {
		return errors.New("there is no extra data for a silent group")
	}
	manager.RLock()
	defer manager.RUnlock()
	ext, ok := respCache[gid]
	if ok {
		return json.Unmarshal(helper.StringToBytes(ext), obj)
	}
	return errors.New("respCache error")
}

func (manager *Manager[CTX]) SetExtra(gid int64, obj any) error {
	if !manager.CanResponse(gid) {
		return errors.New("there is no extra data for a silent group")
	}
	data, err := json.Marshal(obj)
	if err != nil {
		return err
	}
	manager.Lock()
	defer manager.Unlock()
	respCache[gid] = helper.BytesToString(data)
	return manager.D.Insert("__resp", &ResponseGroup{GroupID: gid, Extra: helper.BytesToString(data)})
}
