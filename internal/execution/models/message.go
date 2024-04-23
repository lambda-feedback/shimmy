package models

type Message[T, M any] interface {
	// GetPayload returns the data of the message
	GetPayload() T

	// GetMeta returns the metadata of the message
	GetMeta() M
}

type GenericMessage[T, M any] struct {
	Payload T `json:"data"`
	Meta    M `json:"meta"`
}

func (m *GenericMessage[T, M]) GetPayload() T {
	return m.Payload
}

func (m *GenericMessage[T, M]) GetMeta() M {
	return m.Meta
}
