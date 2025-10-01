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

	"github.com/crolbar/lekvc/lekvc/speex"
	p "github.com/crolbar/lekvc/lekvcs/protocol"
)

const bufferFrames = 0.4 * 48000 * 2

const Address = "localhost:9000"

type Client struct {
	id           uint8
	jitterBuffer *JitterBuffer
	lastSamples  []float32 // for packet loss concealment
}

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

	clients map[uint8]*Client = make(map[uint8]*Client)

	targetFramesize = 1200

	// Simple accumulator for resampling capture input to consistent frame sizes
	captureAccumulator   []float32 = make([]float32, 0, targetFramesize*2)
	captureAccumulatorMu sync.Mutex

	speexPreprocessor *speex.SpeexPreprocessor
)

func captureDevCb(pOutputSample, pInputSamples []byte, framecount uint32) {
	samples := bytesToFloat32(pInputSamples)

	// run speex
	var processed []float32
	if speexPreprocessor != nil {
		samples16 := float32ToInt16(samples)
		_ = speexPreprocessor.Run(samples16)
		processed = int16ToFloat32(samples16)
	} else {
		processed = samples
	}

	if ch == nil || conn == nil {
		return
	}

	captureAccumulatorMu.Lock()
	defer captureAccumulatorMu.Unlock()

	captureAccumulator = append(captureAccumulator, processed...)

	// send frame only if we have enough samples
	for len(captureAccumulator) >= targetFramesize {
		frame := make([]float32, targetFramesize)
		copy(frame, captureAccumulator[:targetFramesize])

		payload := float32ToBytes(frame)

		msg := p.Msg{
			Type:           p.Audio,
			ID:             id,
			PayloadSize:    uint16(len(payload)),
			Payload:        payload,
			ClientName:     name,
			ClientNameSize: uint16(len(name)),
		}

		select {
		case ch <- msg:
		default:
		}

		// remove used samples
		captureAccumulator = captureAccumulator[targetFramesize:]
	}

	// Prevent accumulator from growing indefinitely (keep max 2x targetFramesize)
	if len(captureAccumulator) > targetFramesize*2 {
		captureAccumulator = captureAccumulator[len(captureAccumulator)-targetFramesize:]
	}
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
	client := clients[audioMsg.ID]

	if client != nil && client.jitterBuffer != nil {
		client.jitterBuffer.Add(samples)
	}
}

func readerLoop() {
	defer func() {
		conn.Close()
		close(ch)
		ch = nil
		clear(clients)
		captureAccumulatorMu.Lock()
		captureAccumulator = captureAccumulator[:0]
		captureAccumulatorMu.Unlock()
	}()

	for {
		msg, err := p.ReadMsg(conn)
		// assuming conn is closed
		if err != nil {
			fmt.Printf("\x1b[31merr: %s\x1b[m\n", err.Error())
			break
		}

		// CREATE CLIENT
		{
			// if the msg is not from us or we don't have the client registered
			if _, ok := clients[msg.ID]; msg.ID != id && !ok {
				clients[msg.ID] = &Client{
					id:           msg.ID,
					jitterBuffer: NewJitterBuffer(),
					lastSamples:  make([]float32, 0),
				}

				fmt.Println("created client: ", msg.ID, clients)
			}
		}

		switch msg.Type {
		case p.Audio:
			audioHandle(msg)
		case p.Text:
		case p.ClientJoin:
			fmt.Printf("\x1b[36m%s\x1b[m\n", string(msg.Payload))
		case p.ClientLeave:
			fmt.Printf("\x1b[38;5;124m%s\x1b[m\n", string(msg.Payload))

			// REMOVE CLIENT
			{
				delete(clients, msg.ID)
			}
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

	id, name = InitClient(username)

	fmt.Printf("\x1b[32mConnected to crol.bar:9000 as %s with id %d\x1b[m\n\n", name, id)

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

	speexPreprocessor, err = speex.NewSpeexPreprocessor(targetFramesize, int(sampleRate))
	if err != nil {
		fmt.Printf("\x1b[33mWarning: Failed to init SpeexDSP: %v\x1b[m\n", err)
		fmt.Println("\x1b[33mMake sure libspeexdsp is installed (apt install libspeexdsp-dev)\x1b[m")
		speexPreprocessor = nil // Will run without preprocessing
	}
	defer func() {
		if speexPreprocessor != nil {
			speexPreprocessor.Destroy()
		}
	}()

	captureDev.Start()
	playbackDev.Start()

	go mixer()

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
