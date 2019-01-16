package doctype

import (
	"encoding/json"
	"fmt"
	"github.com/insolar/insolar/logicrunner/goplugin/foundation"
)

type FieldType string
type AttachmentType string

const (
	StringType FieldType = "StringType"
	IntType    FieldType = "IntType"
	BoolType   FieldType = "BoolType"
	DateType   FieldType = "DateType"

	PDF  AttachmentType = "PDF"
	DOCX AttachmentType = "DOCX"
	XML  AttachmentType = "XML"
)

type Field struct {
	Name  string
	Type  FieldType
	Value []byte
}

type Attachment struct {
	Name  string
	Type  FieldType
	Value []byte
}

type DocType struct {
	foundation.BaseContract
	Name        string
	Fields      []Field
	Attachments []Attachment
}

func (docType *DocType) ToJSON() ([]byte, error) {

	documentJSON, err := json.Marshal(docType)
	if err != nil {
		return nil, fmt.Errorf("[ ToJSON ]: %s", err.Error())
	}

	return documentJSON, nil
}

func New(name string, fields []Field, attachments []Attachment) (*DocType, error) {
	return &DocType{
		Name:        name,
		Fields:      fields,
		Attachments: attachments,
	}, nil
}

func NewFromJson(docTypeJson []byte) (*DocType, error) {
	docType := &DocType{}

	err := json.Unmarshal(docTypeJson, docType)
	if err != nil {
		return nil, fmt.Errorf("[ NewFromJson ]: %s", err.Error())
	}

	return docType, nil
}
