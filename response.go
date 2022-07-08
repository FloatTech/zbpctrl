package control

import (
	"errors"
	"strconv"
)

// InitResponse ...
func (manager *Manager[CTX]) initResponse() error {
	return manager.D.Create("__resp", &ResponseGroup{})
}

var respCache = make(map[int64]string)

// Response opens the resp of the gid
func (manager *Manager[CTX]) Response(gid int64) error {
	if manager.CanResponse(gid) {
		return errors.New("group " + strconv.FormatInt(gid, 10) + " already in response")
	}
	manager.Lock()
	defer manager.Unlock()
	respCache[gid] = ""
	return manager.D.Insert("__resp", &ResponseGroup{GroupID: gid})
}

// Silence will drop its extra data
func (manager *Manager[CTX]) Silence(gid int64) error {
	if !manager.CanResponse(gid) {
		return errors.New("group " + strconv.FormatInt(gid, 10) + " already in silence")
	}
	manager.Lock()
	defer manager.Unlock()
	respCache[gid] = "-"
	return manager.D.Del("__resp", "where gid = "+strconv.FormatInt(gid, 10))
}

// CanResponse ...
func (manager *Manager[CTX]) CanResponse(gid int64) bool {
	manager.RLock()
	ext, ok := respCache[0] // all status
	manager.RUnlock()
	if ok && ext != "-" {
		return true
	}
	manager.RLock()
	ext, ok = respCache[gid]
	manager.RUnlock()
	if ok {
		return ext != "-"
	}
	manager.Lock()
	defer manager.Unlock()
	var rsp ResponseGroup
	err := manager.D.Find("__resp", &rsp, "where gid = 0") // all status
	if err == nil && rsp.Extra != "-" {
		respCache[0] = rsp.Extra
		return true
	}
	err = manager.D.Find("__resp", &rsp, "where gid = "+strconv.FormatInt(gid, 10))
	if err != nil {
		respCache[gid] = "-"
		return false
	}
	respCache[gid] = rsp.Extra
	return rsp.Extra != "-"
}
