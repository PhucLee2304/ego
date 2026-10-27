package dto

import identityClient "ego/api/gen/go/identity"

type UserInfo struct {
	ID     string `json:"id"`
	Email  string `json:"email"`
	Name   string `json:"name"`
	Avatar string `json:"avatar"`
}

func ToUserInfoMap(users []*identityClient.UserInfo) map[string]*UserInfo {
	usersByID := make(map[string]*UserInfo, len(users))
	for _, user := range users {
		usersByID[user.Id] = &UserInfo{
			ID:     user.Id,
			Email:  user.Email,
			Name:   user.Name,
			Avatar: user.Avatar,
		}
	}
	return usersByID
}
