package control

// InitResponse ...
func (manager *Manager[CTX]) initUser() error {
	return manager.D.Create("__user", &User{})
}

// CreateOrUpdateUser 创建或修改用户密码
func (manager *Manager[CTX]) CreateOrUpdateUser(u User) error {
	manager.Lock()
	defer manager.Unlock()
	var fu User
	err := manager.D.Find("__user", &fu, "WHERE username = "+u.Username+"AND password = "+u.Password)
	canFind := err == nil && fu.Username == u.Username
	if err != nil || !canFind {
		err = manager.D.Insert("__user", &u)
		return err
	}
	err = manager.D.Del("__user", "WHERE username ="+u.Username)
	if err != nil {
		return err
	}
	return manager.D.Insert("__user", &u)
}

// FindUser 查找用户
func (manager *Manager[CTX]) FindUser(u User) (fu User, err error) {
	manager.RLock()
	defer manager.RUnlock()
	err = manager.D.Find("__user", &fu, "WHERE username = '"+u.Username+"' AND password = '"+u.Password+"'")
	return
}
