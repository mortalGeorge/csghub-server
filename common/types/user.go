package types

import "time"

type CreateUserRequest struct {
	// Display name of the user
	Name string `json:"name"`
	// the login name
	Username string `json:"username"`
	Email    string `json:"email" binding:"email"`
	Phone    string `json:"phone"`
	UUID     string `json:"uuid"`
	// user registered from default login page, from casdoor, etc. Possible values:
	//
	// - "default"
	// - "casdoor"
	RegProvider string `json:"reg_provider"`
}

type UpdateUserRequest struct {
	// Display name of the user
	Nickname *string `json:"name,omitempty"`
	// the login name
	Username string  `json:"-"`
	Email    *string `json:"email,omitempty" binding:"omitnil,email"`
	Phone    *string `json:"phone,omitempty"`
	UUID     *string `json:"uuid,omitempty"`
	// should be updated by admin
	Roles    *[]string `json:"roles,omitempty" example:"[super_user, admin, personal_user]"`
	Avatar   *string   `json:"avatar,omitempty"`
	Homepage *string   `json:"homepage,omitempty"`
	Bio      *string   `json:"bio,omitempty"`
}

type UpdateUserResp struct {
	Username string `json:"username"`
	Email    string `json:"email"`
}

type CreateUserTokenRequest struct {
	Username  string `json:"-" `
	TokenName string `json:"name"`
	// default to csghub
	Application AccessTokenApp `json:"application,omitempty"`
	// default to empty, means full permission
	Permission string    `json:"permission,omitempty"`
	ExpiredAt  time.Time `json:"expired_at"`
}

type CheckAccessTokenReq struct {
	Token string `json:"token" binding:"required"`
	// Optional, if given only check the token belongs to this application
	Application string `json:"application"`
}

type CheckAccessTokenResp struct {
	Token       string         `json:"token"`
	TokenName   string         `json:"token_name"`
	Application AccessTokenApp `json:"application"`
	Permission  string         `json:"permission,omitempty"`
	// the login name
	Username string    `json:"user_name"`
	UserUUID string    `json:"user_uuid"`
	ExpireAt time.Time `json:"expire_at"`
}

type UserDatasetsReq struct {
	Owner       string `json:"owner"`
	CurrentUser string `json:"current_user"`
	PageOpts
}

type (
	UserModelsReq          = UserDatasetsReq
	UserCodesReq           = UserDatasetsReq
	UserSpacesReq          = UserDatasetsReq
	UserCollectionReq      = UserDatasetsReq
	DeleteUserTokenRequest = CreateUserTokenRequest
)

type PageOpts struct {
	Page     int `json:"page"`
	PageSize int `json:"page_size"`
}

type User struct {
	Username    string         `json:"username"`
	Nickname    string         `json:"nickname"`
	Phone       string         `json:"phone"`
	Email       string         `json:"email"`
	UUID        string         `json:"uuid"`
	Avatar      string         `json:"avatar,omitempty"`
	Bio         string         `json:"bio,omitempty"`
	Homepage    string         `json:"homepage,omitempty"`
	Roles       []string       `json:"roles,omitempty"`
	LastLoginAt string         `json:"last_login_at,omitempty"`
	Orgs        []Organization `json:"orgs,omitempty"`
}

type UserLikesRequest struct {
	Username      string `json:"username"`
	Repo_id       int64  `json:"repo_id"`
	Collection_id int64  `json:"collection_id"`
	CurrentUser   string `json:"current_user"`
}

/* for HF compitable apis  */
type WhoamiResponse struct {
	Name  string `json:"name"`
	Email string `json:"email"`
	Auth  Auth   `json:"auth"`
}

type AccessToken struct {
	DisplayName string `json:"displayName,omitempty"`
	Role        string `json:"role,omitempty"`
}

type Auth struct {
	AccessToken `json:"accessToken,omitempty"`
	Type        string `json:"type,omitempty"`
}

type UserRepoReq struct {
	CurrentUser string `json:"current_user"`
	PageOpts
}

type AccessTokenApp string

const (
	AccessTokenAppGit      AccessTokenApp = "git"
	AccessTokenAppCSGHub                  = AccessTokenAppGit
	AccessTokenAppMirror   AccessTokenApp = "mirror"
	AccessTokenAppStarship AccessTokenApp = "starship"
)
