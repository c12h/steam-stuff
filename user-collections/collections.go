package main

type AppNum int32

type CollectionNum int32

type AppInfo struct {
	AppName        string
	AppID          AppNum
	IsHidden       bool
	AppCollections []CollectionNum
}
type UserInfo struct {
	UserNumber int
	UserName   string
	Apps       map[AppNum]AppInfo
}

type CollectionsInfo struct {
	Users              []UserInfo
	CollectionNames    []string // indexed by CollectionNum
	CollectionNamesMap map[string]CollectionNum
}
