package client

import (
	"k8s.io/apimachinery/pkg/runtime"
)

type ObjectAction string

const (
	AddObjectAction    ObjectAction = "add"
	UpdateObjectAction ObjectAction = "upd"
	DeleteObjectAction ObjectAction = "del"
)

type Event struct {
	Object runtime.Object
	Verb   ObjectAction
}
