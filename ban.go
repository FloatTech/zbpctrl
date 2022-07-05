package control

import (
	"crypto/md5"
	"encoding/binary"
	"fmt"
	"strconv"

	log "github.com/sirupsen/logrus"
	"github.com/wdvxdr1123/ZeroBot/utils/helper"
)

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
