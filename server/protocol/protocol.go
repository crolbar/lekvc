package protocol

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"net"
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

	// size of the next four fields
	Size uint16

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

	// MsgSize
	size := 2 + msg.PayloadSize + 2 + msg.ClientNameSize
	if err := binary.Write(buf, binary.LittleEndian, size); err != nil {
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

func ReadMsg(conn net.Conn) (*Msg, error) {
	var (
		msg = Msg{}
		off = 0

		headerBuf = make([]byte, MsgHeaderSize)
		msgBuf    []byte
	)

	// Read header
	{
		n, err := io.ReadFull(conn, headerBuf)
		if err != nil {
			return nil, err
		}
		if n != MsgHeaderSize {
			return nil, errors.New("Packet is not of HeaderSize lenght")
		}
	}

	msg.Type = MsgType(headerBuf[off])
	off += 1

	msg.ID = headerBuf[off]
	off += 1

	msg.Size = binary.LittleEndian.Uint16(headerBuf[off:])
	off += 2

	if msg.Size == 0 {
		return nil, errors.New("invalid msg.Size = 0")
	}

	// Read the rest of the msg
	{
		msgBuf = make([]byte, msg.Size)
		n, err := io.ReadFull(conn, msgBuf)
		if err != nil {
			return nil, err
		}
		if n != int(msg.Size) {
			return nil, errors.New("Packet has wrong msg.Size field or has no payload + clientName.")
		}
		off = 0

		if len(msgBuf) == 0 || len(msgBuf) < int(msg.Size) {
			n := "buggedmf"
			msg.ClientName = n
			msg.ClientNameSize = uint16(len(n))
			return &msg, nil
		}
	}

	msg.PayloadSize = binary.LittleEndian.Uint16(msgBuf[off:])
	off += 2

	msg.Payload = msgBuf[off : off+int(msg.PayloadSize)]
	off += int(msg.PayloadSize)

	msg.ClientNameSize = binary.LittleEndian.Uint16(msgBuf[off:])
	off += 2

	msg.ClientName = string(msgBuf[off : off+int(msg.ClientNameSize)])

	return &msg, nil
}
