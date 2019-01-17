package bprocess

import (
	"encoding/json"
	"fmt"
	procTemplateContract "github.com/insolar/insolar/application/contract/proctemplate"
	"github.com/insolar/insolar/application/proxy/doctype"
	"github.com/insolar/insolar/application/proxy/elemtemplate"
	procTemplateProxy "github.com/insolar/insolar/application/proxy/proctemplate"
	"github.com/insolar/insolar/logicrunner/goplugin/foundation"
)

type BProcess struct {
	foundation.BaseContract
	Name string
}

func (bProcess *BProcess) ToJSON() ([]byte, error) {

	memberJSON, err := json.Marshal(bProcess)
	if err != nil {
		return nil, fmt.Errorf("[ ToJSON ]: %s", err.Error())
	}

	return memberJSON, nil
}

func New(name string) (*BProcess, error) {
	return &BProcess{
		Name: name,
	}, nil
}

// GetProcTemplates processes dump all Business process Process templates
func (bProcess *BProcess) GetProcTemplates() (resultJSON []byte, err error) {

	iterator, err := bProcess.NewChildrenTypedIterator(procTemplateProxy.GetPrototype())
	if err != nil {
		return nil, fmt.Errorf("[ GetProcTemplates ] Can't get children: %s", err.Error())
	}

	res := []procTemplateContract.ProcTemplate{}
	for iterator.HasNext() {
		cref, err := iterator.Next()
		if err != nil {
			return nil, fmt.Errorf("[ GetProcTemplates ] Can't get next child: %s", err.Error())
		}

		procTemplateProxyObject := procTemplateProxy.GetObject(cref)

		procTemplateJSON, err := procTemplateProxyObject.ToJSON()
		if err != nil {
			return nil, fmt.Errorf("[ GetProcTemplates ] Problem with making request: %s", err.Error())
		}

		procTemplateContractObject := procTemplateContract.ProcTemplate{}
		err = json.Unmarshal(procTemplateJSON, &procTemplateContractObject)
		if err != nil {
			return nil, fmt.Errorf("[ GetProcTemplates ] Problem with unmarshal children from response: %s", err.Error())
		}

		res = append(res, procTemplateContractObject)
	}

	resultJSON, err = json.Marshal(res)
	if err != nil {
		return nil, fmt.Errorf("[ GetProcTemplates ] Problem with marshal children: %s", err.Error())
	}

	return resultJSON, nil
}

// GetDocTypes processes dump all Business process Document Types
func (bProcess *BProcess) GetDocTypes() (resultJSON []byte, err error) {

	iterator, err := bProcess.NewChildrenTypedIterator(doctype.GetPrototype())
	if err != nil {
		return nil, fmt.Errorf("[ GetDocTypes ] Can't get children: %s", err.Error())
	}

	type Field struct {
		Name  string
		Type  string
		Value []byte
	}
	type Attachment struct {
		Name  string
		Type  string
		Value []byte
	}
	type DocType struct {
		foundation.BaseContract
		Name        string
		Fields      []Field
		Attachments []Attachment
	}

	res := []DocType{}
	for iterator.HasNext() {
		cref, err := iterator.Next()
		if err != nil {
			return nil, fmt.Errorf("[ GetDocTypes ] Can't get next child: %s", err.Error())
		}

		docTypeProxyObject := doctype.GetObject(cref)

		procTemplateJSON, err := docTypeProxyObject.ToJSON()
		if err != nil {
			return nil, fmt.Errorf("[ GetDocTypes ] Problem with making request: %s", err.Error())
		}

		docTypeContractObject := DocType{}
		err = json.Unmarshal(procTemplateJSON, &docTypeContractObject)
		if err != nil {
			return nil, fmt.Errorf("[ GetDocTypes ] Problem with unmarshal children from response: %s", err.Error())
		}

		res = append(res, docTypeContractObject)
	}

	resultJSON, err = json.Marshal(res)
	if err != nil {
		return nil, fmt.Errorf("[ GetDocTypes ] Problem with marshal children: %s", err.Error())
	}

	return resultJSON, nil
}

// СreateProcTemplate processes create process template request
func (bProcess *BProcess) СreateProcTemplate(name string) (string, error) {

	// create proc template
	procTemplateHolder := procTemplateProxy.New(name)
	pt, err := procTemplateHolder.AsChild(bProcess.GetReference())
	if err != nil {
		return "", fmt.Errorf("[ СreateProcTemplate ] Can't save Process Template as child: %s", err.Error())
	}

	// create start elem template
	startElemTemplateHolder := elemtemplate.New("Start", []string{}, []string{}, []string{})
	startET, err := startElemTemplateHolder.AsChild(pt.GetReference())
	if err != nil {
		return "", fmt.Errorf("[ СreateProcTemplate ] Can't save start Element Template as child: %s", err.Error())
	}
	startETRef := startET.GetReference()
	startETObject := *elemtemplate.GetObject(startETRef)

	// create last elem template
	lastElemTemplateHolder := elemtemplate.New("Last", []string{startETRef.String()}, []string{}, []string{})
	lastET, err := lastElemTemplateHolder.AsChild(pt.GetReference())
	if err != nil {
		return "", fmt.Errorf("[ СreateProcTemplate ] Can't save last Element Template as child: %s", err.Error())
	}
	lastETRef := lastET.GetReference()

	// set elements for process
	if err = startETObject.SetNextElemTemplateSuccessRef(lastETRef.String()); err != nil {
		return "", fmt.Errorf("[ СreateProcTemplate ] Can't Set Next Success Element Template for start element template: %s", err.Error())
	}
	if err = pt.SetElements(startETRef.String(), lastETRef.String()); err != nil {
		return "", fmt.Errorf("[ СreateProcTemplate ] Can't Set Elements for process template: %s", err.Error())
	}

	return pt.GetReference().String(), nil
}
