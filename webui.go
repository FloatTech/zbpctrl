package control

// InitResponse ...
func (manager *Manager[CTX]) initUser() error {
	return manager.D.Create("__user", &User{})
}

// CreateOrUpdateUser 创建或修改用户密码
func (manager *Manager[CTX]) CreateOrUpdateUser(u User) error {
	manager.RLock()
	var fu User
	err := manager.D.Find("__user", &fu, "WHERE username = "+u.Username+"AND password = "+u.Password)
	manager.RUnlock()
	canFind := err == nil && fu.Username == u.Username
	if err != nil || !canFind {
		manager.Lock()
		err = manager.D.Insert("__user", &u)
		manager.Unlock()
		return err
	}
	manager.Lock()
	err = manager.D.Del("__user", "WHERE username ="+u.Username)
	manager.Unlock()
	if err != nil {
		return err
	}
	manager.Lock()
	err = manager.D.Insert("__user", &u)
	manager.Unlock()
	return err
}

// FindUser 查找用户
func (manager *Manager[CTX]) FindUser(u User) (fu User, err error) {
	manager.RLock()
	err = manager.D.Find("__user", &fu, "WHERE username = '"+u.Username+"' AND password = '"+u.Password+"'")
	manager.RUnlock()
	return
}
