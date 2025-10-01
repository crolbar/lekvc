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
	"time"

	"github.com/gen2brain/malgo"

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

	noiseGateV float64 = -65

	targetFramesize = 1200
)

func captureDevCb(pOutputSample, pInputSamples []byte, framecount uint32) {
	// size := int(framecount * uint32(channels))
	// fmt.Println("send: ", size)

	samples := bytesToFloat32(pInputSamples)
	clean := noiseGate(samples, noiseGateV)
	// clean := samples
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
	client := clients[audioMsg.ID]

	if client != nil && client.jitterBuffer != nil {
		client.jitterBuffer.Add(samples)
	}
}

func mixer() {
	// in ms
	var frameTime float32 = (float32(targetFramesize) / float32(sampleRate)) * 1000
	ticker := time.NewTicker(time.Millisecond * time.Duration(frameTime))
	defer ticker.Stop()

	for range ticker.C {
		if len(clients) == 0 {
			continue
		}

		summed := make([]float32, targetFramesize)
		active := 0

		for _, c := range clients {
			if c.jitterBuffer == nil {
				continue
			}

			// Try to get samples from jitter buffer
			samples := c.jitterBuffer.Get(targetFramesize)

			// If no samples available, use packet loss concealment
			if samples == nil {
				samples = c.jitterBuffer.Conceal(targetFramesize, c.lastSamples)
				// Don't count concealed packets as active to reduce their impact
				if len(samples) > 0 {
					// Mix in concealed audio at reduced level
					for i := range samples {
						if i < len(summed) {
							summed[i] += samples[i] * 0.3 // reduce volume of concealed audio
						}
					}
				}
				continue
			}

			// Store last valid samples for future concealment
			if len(samples) > 0 {
				c.lastSamples = make([]float32, len(samples))
				copy(c.lastSamples, samples)
			}

			// Mix in the samples
			for i, s := range samples {
				if i < len(summed) {
					summed[i] += s
				}
			}
			active++
		}

		if active == 0 {
			continue
		}

		// Normalize and apply soft clipping
		for i := range summed {
			summed[i] /= float32(active)

			// Soft clipping to prevent harsh distortion
			if summed[i] > 1 {
				summed[i] = 1
			} else if summed[i] < -1 {
				summed[i] = -1
			}
		}

		ringMu.Lock()
		ring.Write(summed)
		ringMu.Unlock()
	}
}

func readerLoop() {
	defer func() {
		conn.Close()
		close(ch)
		ch = nil
		clear(clients)
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
