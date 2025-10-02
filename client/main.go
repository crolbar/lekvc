package main

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"math"
	"net"
	"os"
	"runtime"
	"strings"
	"sync"

	"github.com/gen2brain/malgo"

	"github.com/crolbar/lekvc/lekvc/preprocessing"
	p "github.com/crolbar/lekvc/lekvcs/protocol"
)

const bufferFrames = 0.4 * 48000 * 2

const Address = "crol.bar:9000"

type Client struct {
	id           uint8
	name         string
	jitterBuffer *JitterBuffer
	lastSamples  []float32 // for packet loss concealment
}

var (
	id          uint8
	name        string
	conn        net.Conn
	isConnected bool

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

	// Audio preprocessing
	audioProcessor *preprocessing.AudioProcessor = preprocessing.NewAudioProcessor(int(sampleRate))
)

func captureDevCb(pOutputSample, pInputSamples []byte, framecount uint32) {
	samples := bytesToFloat32(pInputSamples)

	// Apply professional audio preprocessing
	if audioProcessor != nil {
		samples = audioProcessor.Process(samples)
	}

	if ch == nil || conn == nil {
		return
	}

	captureAccumulatorMu.Lock()
	defer captureAccumulatorMu.Unlock()

	captureAccumulator = append(captureAccumulator, samples...)

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

func textCommandHandle(cmd string) {
	for cstr, c := range commands {
		if cmd == cstr {
			c.f()
		}
	}
	Prompt()
}

func stdinReaderLoop() {
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		if !isConnected {
			break
		}
		text := scanner.Text()
		if strings.HasPrefix(text, "/") {
			textCommandHandle(text)
			continue
		}

		if isConnected {
			Prompt()
		}
		if len(text) == 0 {
			continue
		}
		msg := p.NewMsg(p.Text, id, []byte(text), name)
		ch <- msg
	}
}

func readerLoop() {
	defer func() {
		conn.Close()
		close(ch)
		isConnected = false
		ch = nil
		clear(clients)
		captureAccumulatorMu.Lock()
		captureAccumulator = captureAccumulator[:0]
		captureAccumulatorMu.Unlock()
		// Reset audio processor state
		if audioProcessor != nil {
			audioProcessor.Reset()
		}
	}()

	for {
		msg, err := p.ReadMsg(conn)
		// assuming conn is closed
		if err != nil {
			ChatPrintClient(fmt.Sprintf("\x1b[31merr: %s\x1b[m", err.Error()))
			break
		}

		// CREATE CLIENT
		{
			// if the msg is not from us or we don't have the client registered
			if _, ok := clients[msg.ID]; msg.ID != id && !ok {
				clients[msg.ID] = &Client{
					id:           msg.ID,
					name:         msg.ClientName,
					jitterBuffer: NewJitterBuffer(),
					lastSamples:  make([]float32, 0),
				}
			}
		}

		switch msg.Type {
		case p.Audio:
			audioHandle(msg)
		case p.Text:
			var sender string
			if msg.ID != id {
				sender = msg.ClientName
			} else {
				// server is sending msg back with our id, server telling us something
				sender = "SERVER"
			}
			ChatPrintMsg(sender, msg)
		case p.ClientJoin:
			ChatPrintServer(fmt.Sprintf("\x1b[34m%s\x1b[m", string(msg.Payload)))
		case p.ClientLeave:
			ChatPrintServer(fmt.Sprintf("\x1b[38;5;124m%s\x1b[m", string(msg.Payload)))

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

	ChatPrintClient(fmt.Sprintf("\x1b[32mConnected to crol.bar:9000 as %s with id %d\x1b[m", name, id))

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

	InitCommands()

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

	go mixer()

	for {
		go stdinReaderLoop()
		err = connect()
		if err != nil {
			ChatPrintClient(fmt.Sprintf("\x1b[31merr: %s\x1b[m", err.Error()))
			closeNotifyCH <- true
		}

		isConnected = true

		// wait utill conn is closed
		<-closeNotifyCH

		isConnected = false

		ChatPrintServer("\x1b[33mConnection closed\x1b[m\n")

		fmt.Println("\r\x1b[34m== Press \x1b[32mEnter\x1b[34m twice to try to reconnect ==\x1b[m")
		bufio.NewReader(os.Stdin).ReadBytes('\n')
	}
}
