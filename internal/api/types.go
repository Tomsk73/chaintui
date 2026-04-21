package api

import "time"

type Group struct {
	UID         string    `json:"uid"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	CreateTime  time.Time `json:"createTime"`
	UpdateTime  time.Time `json:"updateTime"`
}

type Identity struct {
	UID         string    `json:"uid"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	CreateTime  time.Time `json:"createTime"`
	UpdateTime  time.Time `json:"updateTime"`
}

type Role struct {
	UID          string    `json:"uid"`
	Name         string    `json:"name"`
	Description  string    `json:"description"`
	Capabilities []string  `json:"capabilities"`
	CreateTime   time.Time `json:"createTime"`
	UpdateTime   time.Time `json:"updateTime"`
}

type RoleBinding struct {
	UID        string    `json:"uid"`
	Identity   string    `json:"identity"`
	Role       string    `json:"role"`
	CreateTime time.Time `json:"createTime"`
	UpdateTime time.Time `json:"updateTime"`
}

type IdentityProvider struct {
	UID         string    `json:"uid"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	CreateTime  time.Time `json:"createTime"`
	UpdateTime  time.Time `json:"updateTime"`
}

type GroupInvite struct {
	UID        string    `json:"uid"`
	Email      string    `json:"email"`
	Role       string    `json:"role"`
	CreateTime time.Time `json:"createTime"`
	ExpiresAt  time.Time `json:"expiresAt"`
}

type Repo struct {
	UID        string    `json:"uid"`
	Name       string    `json:"name"`
	Registry   string    `json:"registry"`
	CreateTime time.Time `json:"createTime"`
	UpdateTime time.Time `json:"updateTime"`
}

type Tag struct {
	UID        string    `json:"uid"`
	Name       string    `json:"name"`
	Digest     string    `json:"digest"`
	CreateTime time.Time `json:"createTime"`
	UpdateTime time.Time `json:"updateTime"`
}

type Advisory struct {
	UID         string    `json:"uid"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Aliases     []string  `json:"aliases"`
	CreateTime  time.Time `json:"createTime"`
	UpdateTime  time.Time `json:"updateTime"`
}
