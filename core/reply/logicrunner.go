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

package reply

import (
	"github.com/insolar/insolar/core"
)

// CallMethod - the most common reply
type CallMethod struct {
	Result []byte
}

// Type returns type of the reply
func (r *CallMethod) Type() core.ReplyType {
	return TypeCallMethod
}

type CallConstructor struct {
	Object *core.RecordRef
}

// Type returns type of the reply
func (r *CallConstructor) Type() core.ReplyType {
	return TypeCallConstructor
}
