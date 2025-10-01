package main

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"math"
	"net"
	"os"
	"runtime"
	"sync"

	"github.com/gen2brain/malgo"

	p "github.com/crolbar/lekvc/lekvcs/protocol"
)

const bufferFrames = 0.4 * 48000 * 2

const Address = "localhost:9000"

var (
	id   uint8
	name string
	conn net.Conn

	username string

	captureDevIdx  = 0
	playbackDevIdx = 0

	captureDevices  []malgo.DeviceInfo
	playbackDevices []malgo.DeviceInfo

	format     = malgo.FormatF32
	channels   = uint32(1)
	sampleRate = uint32(48000)

	malgoCtx    *malgo.AllocatedContext
	playbackDev *malgo.Device
	captureDev  *malgo.Device

	ch            chan p.Msg
	closeNotifyCH chan bool = make(chan bool, 1)

	ring   = NewRingBuffer(bufferFrames * int(channels))
	ringMu sync.Mutex

	noiseGateV float64 = -70
)

func captureDevCb(pOutputSample, pInputSamples []byte, framecount uint32) {
	// size := int(framecount * uint32(channels))
	// fmt.Println("send: ", size)

	samples := bytesToFloat32(pInputSamples)
	clean := noiseGate(samples, noiseGateV)
	applyLowPass(clean)

	// TODO: add noice cancel and other goodies

	if ch == nil || conn == nil {
		return
	}

	payload := float32ToBytes(clean)
	payloadSize := len(payload)

	if payloadSize > math.MaxUint16 {
		panic("too big audio payload")
	}

	msg := p.Msg{
		Type: p.Audio,
		ID:   id,

		PayloadSize: uint16(payloadSize),
		Payload:     payload,

		ClientName:     name,
		ClientNameSize: uint16(len(name)),
	}

	ch <- msg
}

func playbackDevCb(pOutputSample, pInputSamples []byte, framecount uint32) {
	samples := make([]float32, framecount*uint32(channels))

	ringMu.Lock()
	n := ring.Read(samples)
	ringMu.Unlock()

	for i := n; i < len(samples); i++ {
		samples[i] = 0
	}

	for i, v := range samples {
		binary.LittleEndian.PutUint32(
			pOutputSample[i*4:],
			math.Float32bits(v),
		)
	}
}

func audioHandle(audioMsg *p.Msg) {
	samples := bytesToFloat32(audioMsg.Payload)

	ringMu.Lock()
	ring.Write(samples)
	ringMu.Unlock()
}

func readerLoop() {
	defer func() {
		conn.Close()
		close(ch)
		ch = nil
	}()

	for {
		msg, err := p.ReadMsg(conn)
		// assuming conn is closed
		if err != nil {
			fmt.Printf("\x1b[31merr: %s\x1b[m\n", err.Error())
			break
		}

		switch msg.Type {
		case p.Audio:
			audioHandle(msg)
		case p.Text:
		case p.ClientJoin:
			fmt.Printf("\x1b[36m%s\x1b[m\n", string(msg.Payload))
		case p.ClientLeave:
			fmt.Printf("\x1b[38;5;124m%s\x1b[m\n", string(msg.Payload))
		}
	}
}

func writerLoop() {
	for msg := range ch {
		data, _ := p.EncodeMsg(msg)
		conn.Write(data)
	}
	closeNotifyCH <- true
}

func InitClient(name string) (uint8, string) {
	data, err := p.EncodeMsg(
		p.NewMsgNP(p.InitClient, 0, name),
	)
	if err != nil {
		panic(err)
	}

	_, err = conn.Write(data)
	if err != nil {
		panic(err)
	}

	msg, err := p.ReadMsg(conn)
	if err != nil {
		panic(err)
	}

	if msg.Type != p.InitClient {
		panic("wrong msg type recived on init client")
	}

	return msg.ID, msg.ClientName
}

func connect() error {
	var err error

	ch = make(chan p.Msg, 50)

	conn, err = net.Dial("tcp", Address)
	if err != nil {
		close(ch)
		return err
	}

	fmt.Printf("\x1b[32mConnected to crol.bar:9000\x1b[m\n\n")

	id, name = InitClient(username)

	go readerLoop()
	go writerLoop()

	return nil
}

func main() {
	var err error

	if runtime.GOOS == "windows" {
		enableANSI()
		username = os.Getenv("USERNAME")
	} else {
		username = os.Getenv("USER")
	}

	malgoCtx, _ = malgo.InitContext(nil, malgo.ContextConfig{}, nil)
	defer malgoCtx.Free()

	err = SelectDevices(malgoCtx)
	if err != nil {
		panic(err)
	}

	captureDev, playbackDev, err = InitDevices()
	if err != nil {
		panic(err)
	}

	defer captureDev.Uninit()
	defer playbackDev.Uninit()

	captureDev.Start()
	playbackDev.Start()

	for {
		err = connect()
		if err != nil {
			fmt.Printf("\x1b[31merr: %s\x1b[m\n", err.Error())
			closeNotifyCH <- true
		}

		// wait utill conn is closed
		<-closeNotifyCH

		fmt.Printf("\x1b[33mConnection closed\x1b[m\n\n")

		fmt.Println("\x1b[34m== Press \x1b[32mEnter\x1b[34m to try to reconnect ==\x1b[m")
		bufio.NewReader(os.Stdin).ReadBytes('\n')
	}
}
