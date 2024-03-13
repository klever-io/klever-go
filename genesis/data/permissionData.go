package data

import "github.com/klever-io/klever-go/genesis"

type Permission struct {
	Type            int              `json:"type"`
	PermissionName  string           `json:"permissionName"`
	Threshold       int64            `json:"threshold"`
	Operations      string           `json:"operations"`
	Signers         map[string]int64 `json:"signers"`
	SignersBytes    map[string]int64
	ID              int32
	OperationsBytes []byte
}

// PermissionData specify the permission for the address
type PermissionData []*Permission

func (p *PermissionData) Len() int {
	return len(*p)
}

func (p *PermissionData) Get(idx int) genesis.PermissionHandler {
	if p != nil && idx < len(*p) {
		return (*p)[idx]
	}
	return nil
}

// IsInterfaceNil returns if underlying object is true
func (p *PermissionData) IsInterfaceNil() bool {
	return p == nil
}

func (p *Permission) GetID() int32 {
	return p.ID
}

func (p *Permission) GetType() int {
	return p.Type
}

func (p *Permission) GetPermissionName() string {
	return p.PermissionName
}

func (p *Permission) GetThreshold() int64 {
	return p.Threshold
}

func (p *Permission) GetOperations() []byte {
	return p.OperationsBytes
}

func (p *Permission) GetSigners() map[string]int64 {
	return p.SignersBytes
}
