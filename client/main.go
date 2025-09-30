package main

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"

	"github.com/gen2brain/malgo"
)

const bufferFrames = 4800 // 100ms buffer

var (
	captureDevIdx  = 0
	playbackDevIdx = 0

	captureDevices  []malgo.DeviceInfo
	playbackDevices []malgo.DeviceInfo

	audioCh = make(chan []byte, 10)

	format     = malgo.FormatF32
	channels   = uint32(1)
	sampleRate = uint32(48000)

	ring = NewRingBuffer(bufferFrames * int(channels))
)

func captureDevCb(pOutputSample, pInputSamples []byte, framecount uint32) {
	size := int(framecount * uint32(channels))
	samples := bytesToFloat32(pInputSamples, size)

	ring.Write(samples)
}

func playbackDevCb(pOutputSample, pInputSamples []byte, framecount uint32) {
	samples := make([]float32, framecount*uint32(channels))
	n := ring.Read(samples)

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

func main() {
	var err error
	// conn, err := net.Dial("tcp", "localhost:9000")
	// if err != nil {
	// 	panic(err)
	// }

	fmt.Println("loaded listening on :9000")

	ctx, _ := malgo.InitContext(nil, malgo.ContextConfig{}, nil)
	defer ctx.Free()

	err = PrintDevices(ctx)
	if err != nil {
		panic(err)
	}

	captureDevInfo := captureDevices[captureDevIdx]

	var captureDev *malgo.Device

	{
		deviceConfig := malgo.DefaultDeviceConfig(malgo.Capture)
		deviceConfig.Capture.DeviceID = captureDevInfo.ID.Pointer()
		deviceConfig.Capture.Format = format
		deviceConfig.Capture.Channels = channels
		deviceConfig.SampleRate = sampleRate
		deviceConfig.PUserData = nil

		captureCallbacks := malgo.DeviceCallbacks{Data: captureDevCb}
		captureDev, err = malgo.InitDevice(ctx.Context, deviceConfig, captureCallbacks)
	}

	defer captureDev.Uninit()

	playbackDevInfo := playbackDevices[playbackDevIdx]
	var playbackDev *malgo.Device

	{
		deviceConfig := malgo.DefaultDeviceConfig(malgo.Playback)
		deviceConfig.Playback.DeviceID = playbackDevInfo.ID.Pointer()
		deviceConfig.Playback.Format = format
		deviceConfig.Playback.Channels = channels
		deviceConfig.SampleRate = sampleRate
		deviceConfig.PUserData = nil

		captureCallbacks := malgo.DeviceCallbacks{Data: playbackDevCb}

		playbackDev, err = malgo.InitDevice(ctx.Context, deviceConfig, captureCallbacks)
	}

	defer playbackDev.Uninit()

	captureDev.Start()
	playbackDev.Start()

	select {}

	// go func() {
	// 	scanner := bufio.NewScanner(conn)
	// 	for scanner.Scan() {
	// 		fmt.Println(">>", scanner.Text())
	// 	}
	// }()

	// scanner := bufio.NewScanner(os.Stdin)
	// for scanner.Scan() {
	// 	fmt.Fprintln(conn, scanner.Text())
	// }
}

func PrintDevices(ctx *malgo.AllocatedContext) error {
	var err error
	captureDevices, err = ctx.Devices(malgo.Capture)
	if err != nil {
		return err
	}

	fmt.Println("select mic")
	for i, d := range captureDevices {
		fmt.Println(i, d.Name())
	}

	reader := bufio.NewReader(os.Stdin)
	for {
		fmt.Print("Enter device number: ")
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(input)
		idx, err := strconv.Atoi(input)
		if err != nil || idx < 0 || idx >= len(captureDevices) {
			fmt.Println("Invalid number, try again.")
			continue
		}
		captureDevIdx = idx
		break
	}

	playbackDevices, err = ctx.Devices(malgo.Playback)
	if err != nil {
		return err
	}

	fmt.Println()

	fmt.Println("select speaker")
	for i, d := range playbackDevices {
		fmt.Println(i, d.Name())
	}

	reader = bufio.NewReader(os.Stdin)
	for {
		fmt.Print("Enter device number: ")
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(input)
		idx, err := strconv.Atoi(input)
		if err != nil || idx < 0 || idx >= len(playbackDevices) {
			fmt.Println("Invalid number, try again.")
			continue
		}
		playbackDevIdx = idx
		break
	}

	fmt.Println()

	fmt.Printf("using mic %s\n", captureDevices[captureDevIdx].Name())
	fmt.Printf("using speaker %s\n", playbackDevices[playbackDevIdx].Name())

	return nil
}
