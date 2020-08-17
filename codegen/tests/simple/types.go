// Code generated by sysl DO NOT EDIT.
package simple

import (
	"encoding/json"
	"errors"
	"time"

	"github.com/anz-bank/sysl-go/common"
	"github.com/anz-bank/sysl-go/convert"
	"github.com/anz-bank/sysl-go/validator"
	"github.com/rickb777/date"
)

// Reference imports to suppress unused errors
var _ = time.Parse

// Reference imports to suppress unused errors
var _ = date.Parse

// Cat ...
type Cat struct {
	Age   *int64 `json:"age,omitempty"`
	Hunts *bool  `json:"hunts,omitempty"`
	ID    string `json:"id"`
}

// Dog ...
type Dog struct {
	Bark  *bool   `json:"bark,omitempty"`
	Breed *string `json:"breed,omitempty"`
	ID    string  `json:"id"`
}

// Item ...
type Item struct {
	A1   string `json:"A1"`
	A2   string `json:"A2"`
	Name string `json:"-"`
}

// PostRequest ...
type PostRequest struct {
	Bt *bool            `json:"Bt,omitempty"`
	Dt convert.JSONTime `json:"dt"`
	St string           `json:"St"`
}

func (t *PostRequest) UnmarshalJSON(data []byte) error {
	inner := struct {
		Bt *bool             `json:"Bt,omitempty"`
		Dt *convert.JSONTime `json:"dt,omitempty"`
		St *string           `json:"St,omitempty"`
	}{}
	err := json.Unmarshal(data, &inner)
	if err != nil {
		return err
	}
	if inner.Dt == nil {
		return errors.New("dt cannot be nil")
	}

	if inner.St == nil {
		return errors.New("St cannot be nil")
	}

	*t = PostRequest{
		Bt: inner.Bt,
		Dt: *inner.Dt,
		St: *inner.St,
	}
	return nil
}

// Response ...
type Response struct {
	Data ItemSet `json:"Data"`
}

// Status ...
type Status struct {
	StatusField string `json:"statusField"`
}

// Stuff just some stuff

type Stuff struct {
	EmptyStuff     Empty                  `json:"emptyStuff"`
	InnerStuff     string                 `json:"innerStuff"`
	RawTimeStuff   time.Time              `json:"rawTimeStuff"`
	ResponseStuff  Response               `json:"responseStuff"`
	SensitiveStuff common.SensitiveString `json:"sensitiveStuff"`
	SequenceStuff  []Str                  `json:"sequenceStuff,omitempty"`
	TimeStuff      convert.JSONTime       `json:"timeStuff"`
}

// Generate wrapper set type
type ItemSet struct {
	M map[string]Item
}

// GetApiDocsListRequest ...
type GetApiDocsListRequest struct {
}

// GetGetSomeBytesListRequest ...
type GetGetSomeBytesListRequest struct {
}

// GetJustOkAndJustErrorListRequest ...
type GetJustOkAndJustErrorListRequest struct {
}

// GetJustReturnErrorListRequest ...
type GetJustReturnErrorListRequest struct {
}

// GetJustReturnOkListRequest ...
type GetJustReturnOkListRequest struct {
}

// GetOkTypeAndJustErrorListRequest ...
type GetOkTypeAndJustErrorListRequest struct {
}

// GetOopsListRequest ...
type GetOopsListRequest struct {
}

// GetPetaListRequest ...
type GetPetaListRequest struct {
	ID string
}

// GetRawListRequest ...
type GetRawListRequest struct {
	Bt bool
}

// GetRawIntListRequest ...
type GetRawIntListRequest struct {
}

// GetRawStatesListRequest ...
type GetRawStatesListRequest struct {
}

// GetRawIdStatesListRequest ...
type GetRawIdStatesListRequest struct {
	ID string
}

// GetRawStates2ListRequest ...
type GetRawStates2ListRequest struct {
	ID int64
}

// GetSimpleAPIDocsListRequest ...
type GetSimpleAPIDocsListRequest struct {
}

// GetStuffListRequest ...
type GetStuffListRequest struct {
	Dt *convert.JSONTime
	St *string
	Bt *bool
	It int64
}

// PostStuffRequest ...
type PostStuffRequest struct {
	Request Str
}

// *Cat validator
func (s *Cat) Validate() error {
	return validator.Validate(s)
}

// *Dog validator
func (s *Dog) Validate() error {
	return validator.Validate(s)
}

// *Item validator
func (s *Item) Validate() error {
	return validator.Validate(s)
}

// *PostRequest validator
func (s *PostRequest) Validate() error {
	return validator.Validate(s)
}

// *Response validator
func (s *Response) Validate() error {
	return validator.Validate(s)
}

// *Status validator
func (s *Status) Validate() error {
	return validator.Validate(s)
}

// *Stuff validator
func (s *Stuff) Validate() error {
	return validator.Validate(s)
}

// *Item add
func (s *ItemSet) Add(item Item) {
	s.M[item.Name] = item
}

// *Item lookup
func (s *ItemSet) Lookup(Name string) Item {
	return s.M[Name]
}

// Pdf ...
type Pdf []byte

// Integer ...
type Integer int64

// Str ...
type Str string

// Empty ...
type Empty struct {
}

// PetA can be one of following types in runtime:
// Cat
// Dog
type PetA interface {
	// isPetA is identifier method
	isPetA()
}

// isPetA identifies Cat is instance of PetA
func (i Cat) isPetA() {
}

// isPetA identifies Dog is instance of PetA
func (i Dog) isPetA() {
}

// PetB can be one of following types in runtime:
// Cat
// Dog
type PetB interface {
	// isPetB is identifier method
	isPetB()
}

// isPetB identifies Cat is instance of PetB
func (i Cat) isPetB() {
}

// isPetB identifies Dog is instance of PetB
func (i Dog) isPetB() {
}
