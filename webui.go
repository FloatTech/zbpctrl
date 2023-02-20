package control

// InitResponse ...
func (manager *Manager[CTX]) initUser() error {
	return manager.D.Create("__user", &User{})
}

// CreateUser 创建用户
func (manager *Manager[CTX]) CreateUser(u User) error {
	manager.Lock()
	defer manager.Unlock()
	return manager.D.Insert("__user", &u)
}

// CreateOrUpdateUser 创建或修改用户密码
func (manager *Manager[CTX]) CreateOrUpdateUser(u User) error {
	manager.Lock()
	defer manager.Unlock()
	canFind, err := manager.CanFindUser(u)
	if err != nil || !canFind {
		err = manager.CreateUser(u)
		return err
	}
	err = manager.D.Del("__user", "WHERE username ="+u.Username)
	if err != nil {
		return err
	}
	return manager.D.Insert("__user", &u)
}

// CanFindUser 查找用户
func (manager *Manager[CTX]) CanFindUser(u User) (canFind bool, err error) {
	manager.RLock()
	defer manager.RUnlock()
	var fu User
	err = manager.D.Find("__user", &fu, "WHERE username = "+u.Username+"AND password = "+u.Password)
	canFind = err == nil && fu.Username == u.Username
	return
}
