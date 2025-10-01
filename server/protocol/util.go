package protocol

func NewMsg(t MsgType, id uint8, payload []byte, name string) Msg {
	return Msg{
		Type: t,
		ID:   id,

		PayloadSize: uint16(len(payload)),
		Payload:     payload,

		ClientNameSize: uint16(len(name)),
		ClientName:     name,
	}
}

// no payload
func NewMsgNP(t MsgType, id uint8, name string) Msg {
	return Msg{
		Type: t,
		ID:   id,

		ClientNameSize: uint16(len(name)),
		ClientName:     name,
	}
}

// payload
func NewMsgP(t MsgType, id uint8, payload []byte) Msg {
	return Msg{
		Type: t,
		ID:   id,

		PayloadSize: uint16(len(payload)),
		Payload:     payload,
	}
}
