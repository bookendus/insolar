/*
 *    Copyright 2018 Insolar
 *
 *    Licensed under the Apache License, Version 2.0 (the "License");
 *    you may not use this file except in compliance with the License.
 *    You may obtain a copy of the License at
 *
 *        http://www.apache.org/licenses/LICENSE-2.0
 *
 *    Unless required by applicable law or agreed to in writing, software
 *    distributed under the License is distributed on an "AS IS" BASIS,
 *    WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 *    See the License for the specific language governing permissions and
 *    limitations under the License.
 */

package nodedomain

import (
	"crypto/ecdsa"
	"encoding/asn1"

	"github.com/insolar/insolar/core"
	"github.com/insolar/insolar/genesis/experiment/nodedomain/utils"
	"github.com/insolar/insolar/genesis/proxy/noderecord"
	"github.com/insolar/insolar/logicrunner/goplugin/foundation"
)

// NodeDomain holds noderecords
type NodeDomain struct {
	foundation.BaseContract
}

// NewNodeDomain create new NodeDomain
func NewNodeDomain() *NodeDomain {
	return &NodeDomain{}
}

// RegisterNode registers node in system
func (nd *NodeDomain) RegisterNode(pk string, role string) core.RecordRef {
	newRecord := noderecord.NewNodeRecord(pk, role)
	record := newRecord.AsChild(nd.GetReference())
	return record.GetReference()
}

// GetNodeRecord get node record by ref
func (nd *NodeDomain) GetNodeRecord(ref core.RecordRef) *noderecord.NodeRecord {
	return noderecord.GetObject(ref)
}

// RemoveNode deletes node from registry
func (nd *NodeDomain) RemoveNode(nodeRef core.RecordRef) {
	node := noderecord.GetObject(nodeRef)
	node.Destroy()
}

func (nd *NodeDomain) IsAuthorized(nodeRef core.RecordRef, seed []byte, signatureRaw []byte) bool {

	var ecdsaPair utils.EcdsaPair
	rest, err := asn1.Unmarshal(signatureRaw, &ecdsaPair)
	if err != nil || len(rest) != 0 {
		panic(err)
	}

	nodeR := nd.GetNodeRecord(nodeRef)
	savedKey, err := utils.DeserializePublicKey(nodeR.GetPublicKey())
	if err != nil {
		return false
	}

	hash := utils.MakeHash(seed)

	return ecdsa.Verify(savedKey, hash[:], ecdsaP.First, ecdsaP.Second)
}
