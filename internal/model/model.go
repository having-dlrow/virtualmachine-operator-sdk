package model

import v1 "github.com/example/virtualmachine/api/v1"

type VMRequest struct {
	Name string                `json:"name"`
	Spec v1.VirtualMachineSpec `json:"spec"`
}
