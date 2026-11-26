package authctx

import (
	"github.com/KevinZonda/Panel/models/dbmod"
	"github.com/gin-gonic/gin"
)

const (
	userKey    = "panel.current_user"
	sessionKey = "panel.current_session"
)

func SetUser(c *gin.Context, user *dbmod.User) {
	c.Set(userKey, user)
}

func GetUser(c *gin.Context) (*dbmod.User, bool) {
	v, ok := c.Get(userKey)
	if !ok {
		return nil, false
	}
	user, ok := v.(*dbmod.User)
	return user, ok
}

func MustUser(c *gin.Context) *dbmod.User {
	user, _ := GetUser(c)
	return user
}

func SetSession(c *gin.Context, session *dbmod.AuthSession) {
	c.Set(sessionKey, session)
}

func GetSession(c *gin.Context) (*dbmod.AuthSession, bool) {
	v, ok := c.Get(sessionKey)
	if !ok {
		return nil, false
	}
	session, ok := v.(*dbmod.AuthSession)
	return session, ok
}

func MustSession(c *gin.Context) *dbmod.AuthSession {
	session, _ := GetSession(c)
	return session
}
