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

package platformpolicy

import (
	"crypto"

	"github.com/insolar/insolar/core"
	"github.com/insolar/insolar/platformpolicy/internal/hash"
	"github.com/insolar/insolar/platformpolicy/internal/sign"
)

type platformCryptographyScheme struct {
	HashProvider hash.AlgorithmProvider `inject:""`
	SignProvider sign.AlgorithmProvider `inject:""`
}

func (pcs *platformCryptographyScheme) ReferenceHasher() core.Hasher {
	return pcs.HashProvider.Hash224bits()
}

func (pcs *platformCryptographyScheme) IntegrityHasher() core.Hasher {
	return pcs.HashProvider.Hash512bits()
}

}
