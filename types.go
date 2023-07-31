package main

// shadow  message definitions
// reference to aws iot
import (
	"time"
)

type Shadow struct {
	ThingId  string    `json:"thingId"`
	State    StateDR   `json:"state"`
	Metadata Metadata  `json:"metadata"`
	Version  int64     `json:"version"`
	Tags     TagsValue `json:"tags"`

	UpdatedAt time.Time `json:"updatedAt"`
	CreatedAt time.Time `json:"createdAt"`
}

type ShadowWithStatus struct {
	Connected      *bool      `json:"connected,omitempty"`
	ConnectedAt    *time.Time `json:"connectedAt,omitempty"`
	DisconnectedAt *time.Time `json:"disconnectedAt,omitempty"`
	RemoteAddr     string     `json:"remoteAddr,omitempty"`
	Shadow
}

// ErrResp Error response document
// code — An HTTP response code that indicates the type of error.
// message — A text message that provides additional information.
// timestamp — The date and time the response was generated by tio. This property is not present in all error response documents.
// clientToken — Present only if a client token was used in the published message.
type ErrResp struct {
	Code        int    `json:"code"`
	Message     string `json:"message"`
	Timestamp   int64  `json:"timestamp"`
	ClientToken string `json:"clientToken"`
}

// StateReq Publish a request state document to update the device's shadow
// A request state document has the following format:
//
// state — Updates affect only the fields specified. Typically, you'll use either the desired or the reported property, but not both in the same request.
//
//	desired — The state properties and values requested to be updated in the device.
//	reported — The state properties and values reported by the device.
//
// clientToken — If used, you can match the request and corresponding response by the client token.
// version — If used, the Device Shadow service processes the update only if the specified version matches the latest version it has.
type StateReq struct {
	State       StateDR `json:"state"`
	ClientToken string  `json:"clientToken"`
	Version     int64   `json:"version"`
}

// TagsReq Publish a request state document to set tag for the device's shadow
// version — If used, the Device Shadow service processes the update only if the specified version matches the latest version it has.
type TagsReq struct {
	Tags    TagsValue `json:"tags"`
	Version int64     `json:"version"`
}

// GetReq Publish a request to get device's shadow
type GetReq struct {
	ClientToken string // optional
}

// StateAcceptedResp  tio publishes a response shadow document to this topic when returning the device's shadow:
// /accepted response state document
type StateAcceptedResp struct {
	State       StateDRD `json:"state"`
	Metadata    Metadata `json:"metadata"`
	Timestamp   int64    `json:"timestamp"`
	ClientToken string   `json:"clientToken"`
	Version     int64    `json:"version"`
}

// StateUpdatedNotice
// /documents response state document
// Response state document properties
// previous — After a successful update, contains the state of the object before the update.
// current — After a successful update, contains the state of the object after the update.
// state
//
//	reported — Present only if a thing reported any data in the reported section and contains only fields that were in the request state document.
//	desired — Present only if a device reported any data in the desired section and contains only fields that were in the request state document.
//	delta — Present only if the desired data differs from the shadow's current reported data.
//
// metadata — Contains the timestamps for each attribute in the desired and reported sections so that you can determine when the state was updated.
// timestamp — The Epoch date and time the response was generated by tio.
// clientToken — Present only if a client token was used when publishing valid JSON to the /update topic.
// version — The current version of the document for the device's shadow shared in tio. It is increased by one over the previous version of the document.
type StateUpdatedNotice struct {
	Previous    StatePrevious `json:"previous"`
	Current     StateCurrent  `json:"current"`
	Timestamp   int64         `json:"timestamp"`
	ClientToken string        `json:"clientToken"`
}

// DeltaStateNotice publishes a response state document when it accepts a change for the device's shadow,
// and the request state document contains different values for desired and reported states:
// /delta response state document
type DeltaStateNotice struct {
	State       StateValue `json:"state"`
	Metadata    MetaValue  `json:"metadata"`
	Timestamp   int64      `json:"timestamp"`
	ClientToken string     `json:"clientToken"`
	Version     int64      `json:"version"`
}

/**
middle types for state
*/

type StateDR struct {
	Desired  StateValue `json:"desired"`
	Reported StateValue `json:"reported"`
}

type StateDRD struct {
	Desired  StateValue `json:"desired,omitempty"`
	Reported StateValue `json:"reported,omitempty"`
	Delta    StateValue `json:"delta,omitempty"`
}

type StatePrevious struct {
	State    StateDR  `json:"state"`
	Metadata Metadata `json:"metadata"`
	Version  int64    `json:"version"`
}
type StateCurrent StatePrevious

// Metadata metadata for state
// same structure as state, but leaf node is metadata object like timestamp --- the state field last update time
type Metadata struct {
	Desired  MetaValue `json:"desired,omitempty"`
	Reported MetaValue `json:"reported,omitempty"`
}
type MetaValue map[string]any
type MetaTimestamp struct {
	Timestamp int64 `json:"timestamp"` // Unix timestamp in Millisecond
}

type StateValue map[string]any

type TagsValue map[string]any

func NewStateDR() StateDR {
	return StateDR{Desired: StateValue{}, Reported: StateValue{}}
}
func NewStateDRD() StateDRD {
	return StateDRD{Desired: StateValue{}, Reported: StateValue{}, Delta: StateValue{}}
}
func NewMetadata() Metadata {
	return Metadata{Desired: MetaValue{}, Reported: MetaValue{}}
}

func IsStateValueEmpty(s StateValue) bool {
	return len(s) == 0
}

type MethodResp struct {
	ClientToken string `json:"clientToken,omitempty"`
	Data        any    `json:"data,omitempty"`
	Code        int    `json:"code"`
	Message     string `json:"message"`
}
type MethodReq struct {
	ClientToken string `json:"clientToken,omitempty"`
	Data        any    `json:"data,omitempty"`
}