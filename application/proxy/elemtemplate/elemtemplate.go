package elemtemplate

import (
	"github.com/insolar/insolar/core"
	"github.com/insolar/insolar/logicrunner/goplugin/foundation"
	"github.com/insolar/insolar/logicrunner/goplugin/proxyctx"
)

// PrototypeReference to prototype of this contract
// error checking hides in generator
var PrototypeReference, _ = core.NewRefFromBase58("1111xy2cUeFVFqkqRjSuYeZEvgVy47f7kDMAcW4usF.11111111111111111111111111111111")

// ElemTemplate holds proxy type
type ElemTemplate struct {
	Reference core.RecordRef
	Prototype core.RecordRef
	Code      core.RecordRef
}

// ContractConstructorHolder holds logic with object construction
type ContractConstructorHolder struct {
	constructorName string
	argsSerialized  []byte
}

// AsChild saves object as child
func (r *ContractConstructorHolder) AsChild(objRef core.RecordRef) (*ElemTemplate, error) {
	ref, err := proxyctx.Current.SaveAsChild(objRef, *PrototypeReference, r.constructorName, r.argsSerialized)
	if err != nil {
		return nil, err
	}
	return &ElemTemplate{Reference: ref}, nil
}

// AsDelegate saves object as delegate
func (r *ContractConstructorHolder) AsDelegate(objRef core.RecordRef) (*ElemTemplate, error) {
	ref, err := proxyctx.Current.SaveAsDelegate(objRef, *PrototypeReference, r.constructorName, r.argsSerialized)
	if err != nil {
		return nil, err
	}
	return &ElemTemplate{Reference: ref}, nil
}

// GetObject returns proxy object
func GetObject(ref core.RecordRef) (r *ElemTemplate) {
	return &ElemTemplate{Reference: ref}
}

// GetPrototype returns reference to the prototype
func GetPrototype() core.RecordRef {
	return *PrototypeReference
}

// GetImplementationFrom returns proxy to delegate of given type
func GetImplementationFrom(object core.RecordRef) (*ElemTemplate, error) {
	ref, err := proxyctx.Current.GetDelegate(object, *PrototypeReference)
	if err != nil {
		return nil, err
	}
	return GetObject(ref), nil
}

// New is constructor
func New(name string, previousElements []string, nextElementTemplateSuccess []string, nextElementTemplateFail []string) *ContractConstructorHolder {
	var args [4]interface{}
	args[0] = name
	args[1] = previousElements
	args[2] = nextElementTemplateSuccess
	args[3] = nextElementTemplateFail

	var argsSerialized []byte
	err := proxyctx.Current.Serialize(args, &argsSerialized)
	if err != nil {
		panic(err)
	}

	return &ContractConstructorHolder{constructorName: "New", argsSerialized: argsSerialized}
}

// GetReference returns reference of the object
func (r *ElemTemplate) GetReference() core.RecordRef {
	return r.Reference
}

// GetPrototype returns reference to the code
func (r *ElemTemplate) GetPrototype() (core.RecordRef, error) {
	if r.Prototype.IsEmpty() {
		ret := [2]interface{}{}
		var ret0 core.RecordRef
		ret[0] = &ret0
		var ret1 *foundation.Error
		ret[1] = &ret1

		res, err := proxyctx.Current.RouteCall(r.Reference, true, "GetPrototype", make([]byte, 0), *PrototypeReference)
		if err != nil {
			return ret0, err
		}

		err = proxyctx.Current.Deserialize(res, &ret)
		if err != nil {
			return ret0, err
		}

		if ret1 != nil {
			return ret0, ret1
		}

		r.Prototype = ret0
	}

	return r.Prototype, nil

}

// GetCode returns reference to the code
func (r *ElemTemplate) GetCode() (core.RecordRef, error) {
	if r.Code.IsEmpty() {
		ret := [2]interface{}{}
		var ret0 core.RecordRef
		ret[0] = &ret0
		var ret1 *foundation.Error
		ret[1] = &ret1

		res, err := proxyctx.Current.RouteCall(r.Reference, true, "GetCode", make([]byte, 0), *PrototypeReference)
		if err != nil {
			return ret0, err
		}

		err = proxyctx.Current.Deserialize(res, &ret)
		if err != nil {
			return ret0, err
		}

		if ret1 != nil {
			return ret0, ret1
		}

		r.Code = ret0
	}

	return r.Code, nil
}

// SetPreviousElemTemplateRef is proxy generated method
func (r *ElemTemplate) SetPreviousElemTemplateRef(previousElemTemplateRef string) error {
	var args [1]interface{}
	args[0] = previousElemTemplateRef

	var argsSerialized []byte

	ret := [1]interface{}{}
	var ret0 *foundation.Error
	ret[0] = &ret0

	err := proxyctx.Current.Serialize(args, &argsSerialized)
	if err != nil {
		return err
	}

	res, err := proxyctx.Current.RouteCall(r.Reference, true, "SetPreviousElemTemplateRef", argsSerialized, *PrototypeReference)
	if err != nil {
		return err
	}

	err = proxyctx.Current.Deserialize(res, &ret)
	if err != nil {
		return err
	}

	if ret0 != nil {
		return ret0
	}
	return nil
}

// SetPreviousElemTemplateRefNoWait is proxy generated method
func (r *ElemTemplate) SetPreviousElemTemplateRefNoWait(previousElemTemplateRef string) error {
	var args [1]interface{}
	args[0] = previousElemTemplateRef

	var argsSerialized []byte

	err := proxyctx.Current.Serialize(args, &argsSerialized)
	if err != nil {
		return err
	}

	_, err = proxyctx.Current.RouteCall(r.Reference, false, "SetPreviousElemTemplateRef", argsSerialized, *PrototypeReference)
	if err != nil {
		return err
	}

	return nil
}

// SetNextElemTemplateSuccessRef is proxy generated method
func (r *ElemTemplate) SetNextElemTemplateSuccessRef(nextElemTemplateSuccessRef string) error {
	var args [1]interface{}
	args[0] = nextElemTemplateSuccessRef

	var argsSerialized []byte

	ret := [1]interface{}{}
	var ret0 *foundation.Error
	ret[0] = &ret0

	err := proxyctx.Current.Serialize(args, &argsSerialized)
	if err != nil {
		return err
	}

	res, err := proxyctx.Current.RouteCall(r.Reference, true, "SetNextElemTemplateSuccessRef", argsSerialized, *PrototypeReference)
	if err != nil {
		return err
	}

	err = proxyctx.Current.Deserialize(res, &ret)
	if err != nil {
		return err
	}

	if ret0 != nil {
		return ret0
	}
	return nil
}

// SetNextElemTemplateSuccessRefNoWait is proxy generated method
func (r *ElemTemplate) SetNextElemTemplateSuccessRefNoWait(nextElemTemplateSuccessRef string) error {
	var args [1]interface{}
	args[0] = nextElemTemplateSuccessRef

	var argsSerialized []byte

	err := proxyctx.Current.Serialize(args, &argsSerialized)
	if err != nil {
		return err
	}

	_, err = proxyctx.Current.RouteCall(r.Reference, false, "SetNextElemTemplateSuccessRef", argsSerialized, *PrototypeReference)
	if err != nil {
		return err
	}

	return nil
}

// SetNextElemTemplateFailRef is proxy generated method
func (r *ElemTemplate) SetNextElemTemplateFailRef(nextElemTemplateFailRef string) error {
	var args [1]interface{}
	args[0] = nextElemTemplateFailRef

	var argsSerialized []byte

	ret := [1]interface{}{}
	var ret0 *foundation.Error
	ret[0] = &ret0

	err := proxyctx.Current.Serialize(args, &argsSerialized)
	if err != nil {
		return err
	}

	res, err := proxyctx.Current.RouteCall(r.Reference, true, "SetNextElemTemplateFailRef", argsSerialized, *PrototypeReference)
	if err != nil {
		return err
	}

	err = proxyctx.Current.Deserialize(res, &ret)
	if err != nil {
		return err
	}

	if ret0 != nil {
		return ret0
	}
	return nil
}

// SetNextElemTemplateFailRefNoWait is proxy generated method
func (r *ElemTemplate) SetNextElemTemplateFailRefNoWait(nextElemTemplateFailRef string) error {
	var args [1]interface{}
	args[0] = nextElemTemplateFailRef

	var argsSerialized []byte

	err := proxyctx.Current.Serialize(args, &argsSerialized)
	if err != nil {
		return err
	}

	_, err = proxyctx.Current.RouteCall(r.Reference, false, "SetNextElemTemplateFailRef", argsSerialized, *PrototypeReference)
	if err != nil {
		return err
	}

	return nil
}
