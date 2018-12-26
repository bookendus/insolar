package proctemplate

import (
	"github.com/insolar/insolar/core"
	"github.com/insolar/insolar/logicrunner/goplugin/foundation"
	"github.com/insolar/insolar/logicrunner/goplugin/proxyctx"
)

// PrototypeReference to prototype of this contract
// error checking hides in generator
var PrototypeReference, _ = core.NewRefFromBase58("11113K9JF912tAiJoYotJ2Y1QVK9hqqDjFAgWPED1qh.11111111111111111111111111111111")

// ProcTemplate holds proxy type
type ProcTemplate struct {
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
func (r *ContractConstructorHolder) AsChild(objRef core.RecordRef) (*ProcTemplate, error) {
	ref, err := proxyctx.Current.SaveAsChild(objRef, *PrototypeReference, r.constructorName, r.argsSerialized)
	if err != nil {
		return nil, err
	}
	return &ProcTemplate{Reference: ref}, nil
}

// AsDelegate saves object as delegate
func (r *ContractConstructorHolder) AsDelegate(objRef core.RecordRef) (*ProcTemplate, error) {
	ref, err := proxyctx.Current.SaveAsDelegate(objRef, *PrototypeReference, r.constructorName, r.argsSerialized)
	if err != nil {
		return nil, err
	}
	return &ProcTemplate{Reference: ref}, nil
}

// GetObject returns proxy object
func GetObject(ref core.RecordRef) (r *ProcTemplate) {
	return &ProcTemplate{Reference: ref}
}

// GetPrototype returns reference to the prototype
func GetPrototype() core.RecordRef {
	return *PrototypeReference
}

// GetImplementationFrom returns proxy to delegate of given type
func GetImplementationFrom(object core.RecordRef) (*ProcTemplate, error) {
	ref, err := proxyctx.Current.GetDelegate(object, *PrototypeReference)
	if err != nil {
		return nil, err
	}
	return GetObject(ref), nil
}

// New is constructor
func New(name string) *ContractConstructorHolder {
	var args [1]interface{}
	args[0] = name

	var argsSerialized []byte
	err := proxyctx.Current.Serialize(args, &argsSerialized)
	if err != nil {
		panic(err)
	}

	return &ContractConstructorHolder{constructorName: "New", argsSerialized: argsSerialized}
}

// GetReference returns reference of the object
func (r *ProcTemplate) GetReference() core.RecordRef {
	return r.Reference
}

// GetPrototype returns reference to the code
func (r *ProcTemplate) GetPrototype() (core.RecordRef, error) {
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
func (r *ProcTemplate) GetCode() (core.RecordRef, error) {
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

// ToJSON is proxy generated method
func (r *ProcTemplate) ToJSON() ([]byte, error) {
	var args [0]interface{}

	var argsSerialized []byte

	ret := [2]interface{}{}
	var ret0 []byte
	ret[0] = &ret0
	var ret1 *foundation.Error
	ret[1] = &ret1

	err := proxyctx.Current.Serialize(args, &argsSerialized)
	if err != nil {
		return ret0, err
	}

	res, err := proxyctx.Current.RouteCall(r.Reference, true, "ToJSON", argsSerialized, *PrototypeReference)
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
	return ret0, nil
}

// ToJSONNoWait is proxy generated method
func (r *ProcTemplate) ToJSONNoWait() error {
	var args [0]interface{}

	var argsSerialized []byte

	err := proxyctx.Current.Serialize(args, &argsSerialized)
	if err != nil {
		return err
	}

	_, err = proxyctx.Current.RouteCall(r.Reference, false, "ToJSON", argsSerialized, *PrototypeReference)
	if err != nil {
		return err
	}

	return nil
}

// CreateDocument is proxy generated method
func (r *ProcTemplate) CreateDocument(name string, docTypeReferenceStr string) (string, error) {
	var args [2]interface{}
	args[0] = name
	args[1] = docTypeReferenceStr

	var argsSerialized []byte

	ret := [2]interface{}{}
	var ret0 string
	ret[0] = &ret0
	var ret1 *foundation.Error
	ret[1] = &ret1

	err := proxyctx.Current.Serialize(args, &argsSerialized)
	if err != nil {
		return ret0, err
	}

	res, err := proxyctx.Current.RouteCall(r.Reference, true, "CreateDocument", argsSerialized, *PrototypeReference)
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
	return ret0, nil
}

// CreateDocumentNoWait is proxy generated method
func (r *ProcTemplate) CreateDocumentNoWait(name string, docTypeReferenceStr string) error {
	var args [2]interface{}
	args[0] = name
	args[1] = docTypeReferenceStr

	var argsSerialized []byte

	err := proxyctx.Current.Serialize(args, &argsSerialized)
	if err != nil {
		return err
	}

	_, err = proxyctx.Current.RouteCall(r.Reference, false, "CreateDocument", argsSerialized, *PrototypeReference)
	if err != nil {
		return err
	}

	return nil
}

// GetDocuments is proxy generated method
func (r *ProcTemplate) GetDocuments() ([]byte, error) {
	var args [0]interface{}

	var argsSerialized []byte

	ret := [2]interface{}{}
	var ret0 []byte
	ret[0] = &ret0
	var ret1 *foundation.Error
	ret[1] = &ret1

	err := proxyctx.Current.Serialize(args, &argsSerialized)
	if err != nil {
		return ret0, err
	}

	res, err := proxyctx.Current.RouteCall(r.Reference, true, "GetDocuments", argsSerialized, *PrototypeReference)
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
	return ret0, nil
}

// GetDocumentsNoWait is proxy generated method
func (r *ProcTemplate) GetDocumentsNoWait() error {
	var args [0]interface{}

	var argsSerialized []byte

	err := proxyctx.Current.Serialize(args, &argsSerialized)
	if err != nil {
		return err
	}

	_, err = proxyctx.Current.RouteCall(r.Reference, false, "GetDocuments", argsSerialized, *PrototypeReference)
	if err != nil {
		return err
	}

	return nil
}

// CreateStageTemplate is proxy generated method
func (r *ProcTemplate) CreateStageTemplate(name string, previousElemTemplatesRefs []string, nextElementTemplateSuccessRefs []string, nextElementTemplateFailRefs []string, participantsRef string, expirationDate string) (string, error) {
	var args [6]interface{}
	args[0] = name
	args[1] = previousElemTemplatesRefs
	args[2] = nextElementTemplateSuccessRefs
	args[3] = nextElementTemplateFailRefs
	args[4] = participantsRef
	args[5] = expirationDate

	var argsSerialized []byte

	ret := [2]interface{}{}
	var ret0 string
	ret[0] = &ret0
	var ret1 *foundation.Error
	ret[1] = &ret1

	err := proxyctx.Current.Serialize(args, &argsSerialized)
	if err != nil {
		return ret0, err
	}

	res, err := proxyctx.Current.RouteCall(r.Reference, true, "CreateStageTemplate", argsSerialized, *PrototypeReference)
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
	return ret0, nil
}

// CreateStageTemplateNoWait is proxy generated method
func (r *ProcTemplate) CreateStageTemplateNoWait(name string, previousElemTemplatesRefs []string, nextElementTemplateSuccessRefs []string, nextElementTemplateFailRefs []string, participantsRef string, expirationDate string) error {
	var args [6]interface{}
	args[0] = name
	args[1] = previousElemTemplatesRefs
	args[2] = nextElementTemplateSuccessRefs
	args[3] = nextElementTemplateFailRefs
	args[4] = participantsRef
	args[5] = expirationDate

	var argsSerialized []byte

	err := proxyctx.Current.Serialize(args, &argsSerialized)
	if err != nil {
		return err
	}

	_, err = proxyctx.Current.RouteCall(r.Reference, false, "CreateStageTemplate", argsSerialized, *PrototypeReference)
	if err != nil {
		return err
	}

	return nil
}

// CreateConditionRouterTemplate is proxy generated method
func (r *ProcTemplate) CreateConditionRouterTemplate(name string, previousElemTemplatesRefs []string, nextElementTemplateSuccess []string, nextElementTemplateFail []string) (string, error) {
	var args [4]interface{}
	args[0] = name
	args[1] = previousElemTemplatesRefs
	args[2] = nextElementTemplateSuccess
	args[3] = nextElementTemplateFail

	var argsSerialized []byte

	ret := [2]interface{}{}
	var ret0 string
	ret[0] = &ret0
	var ret1 *foundation.Error
	ret[1] = &ret1

	err := proxyctx.Current.Serialize(args, &argsSerialized)
	if err != nil {
		return ret0, err
	}

	res, err := proxyctx.Current.RouteCall(r.Reference, true, "CreateConditionRouterTemplate", argsSerialized, *PrototypeReference)
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
	return ret0, nil
}

// CreateConditionRouterTemplateNoWait is proxy generated method
func (r *ProcTemplate) CreateConditionRouterTemplateNoWait(name string, previousElemTemplatesRefs []string, nextElementTemplateSuccess []string, nextElementTemplateFail []string) error {
	var args [4]interface{}
	args[0] = name
	args[1] = previousElemTemplatesRefs
	args[2] = nextElementTemplateSuccess
	args[3] = nextElementTemplateFail

	var argsSerialized []byte

	err := proxyctx.Current.Serialize(args, &argsSerialized)
	if err != nil {
		return err
	}

	_, err = proxyctx.Current.RouteCall(r.Reference, false, "CreateConditionRouterTemplate", argsSerialized, *PrototypeReference)
	if err != nil {
		return err
	}

	return nil
}
