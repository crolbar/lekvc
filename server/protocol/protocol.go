package protocol

import (
	"bytes"
	"encoding/binary"
	// "net"
)

type MsgType uint8

const (
	// client + server
	Audio MsgType = iota
	Text
	InitClient

	// server sender only
	ClientJoin
	ClientLeave
)

const MsgHeaderSize = 1 + 1 + 2

type Msg struct {
	Type MsgType
	// Client id, to whom belongs, text, audio, leave, join
	// use id = 0 for InitClient, used to get client id on client
	ID uint8

	// optional in InitClient
	PayloadSize uint16
	Payload     []byte

	// optional in InitClient, Audio
	ClientNameSize uint16
	ClientName     string
}

func EncodeMsg(msg Msg) ([]byte, error) {
	buf := new(bytes.Buffer)

	// TYPE
	if err := binary.Write(buf, binary.LittleEndian, msg.Type); err != nil {
		return nil, err
	}

	// ID
	if err := binary.Write(buf, binary.LittleEndian, msg.ID); err != nil {
		return nil, err
	}

	// PAYLOAD SIZE
	if err := binary.Write(buf, binary.LittleEndian, msg.PayloadSize); err != nil {
		return nil, err
	}

	// PAYLOAD
	if _, err := buf.Write(msg.Payload); err != nil {
		return nil, err
	}

	// CLIENT NAME SIZE
	if err := binary.Write(buf, binary.LittleEndian, msg.ClientNameSize); err != nil {
		return nil, err
	}

	// CLIENT NAME
	if _, err := buf.Write([]byte(msg.ClientName)); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

func GetPayloadSize(data []byte) uint16 {
	return binary.LittleEndian.Uint16(data[1+1:])
}

func GetClientNameSize(data []byte, PayloadSize uint16) uint16 {
	return binary.LittleEndian.Uint16(data[MsgHeaderSize+PayloadSize:])
}

func GetPayload(data []byte, PayloadSize uint16) []byte {
	return data[MsgHeaderSize : MsgHeaderSize+PayloadSize]
}

func GetClientName(data []byte, PayloadSize uint16, ClientNameSize uint16) []byte {
	return data[MsgHeaderSize+PayloadSize+2 : MsgHeaderSize+PayloadSize+2+ClientNameSize]
}

// func ReadFullMsg(conn net.Conn) ([]byte, error) {
// 	kk
// }

func DecodeMsg(data []byte) (Msg, error) {
	var (
		msg = Msg{}
		off = 0
	)

	msg.Type = MsgType(data[off])
	off += 1

	msg.ID = data[off]
	off += 1

	msg.PayloadSize = binary.LittleEndian.Uint16(data[off:])
	off += 2

	msg.Payload = data[off : off+int(msg.PayloadSize)]
	off += int(msg.PayloadSize)

	msg.ClientNameSize = binary.LittleEndian.Uint16(data[off:])
	off += 2

	msg.ClientName = string(data[off : off+int(msg.ClientNameSize)])

	return msg, nil
}
