package main

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/gen2brain/malgo"
)

func InitDevices() (capture *malgo.Device, playback *malgo.Device, err error) {
	captureDevInfo := captureDevices[captureDevIdx]

	{
		deviceConfig := malgo.DefaultDeviceConfig(malgo.Capture)
		deviceConfig.Capture.DeviceID = captureDevInfo.ID.Pointer()
		deviceConfig.Capture.Format = format
		deviceConfig.Capture.Channels = channels
		deviceConfig.SampleRate = sampleRate
		deviceConfig.PUserData = nil

		captureCallbacks := malgo.DeviceCallbacks{Data: captureDevCb}
		capture, err = malgo.InitDevice(malgoCtx.Context, deviceConfig, captureCallbacks)
		if err != nil {
			return nil, nil, err
		}
	}

	playbackDevInfo := playbackDevices[playbackDevIdx]

	{
		deviceConfig := malgo.DefaultDeviceConfig(malgo.Playback)
		deviceConfig.Playback.DeviceID = playbackDevInfo.ID.Pointer()
		deviceConfig.Playback.Format = format
		deviceConfig.Playback.Channels = channels
		deviceConfig.SampleRate = sampleRate
		deviceConfig.PUserData = nil

		captureCallbacks := malgo.DeviceCallbacks{Data: playbackDevCb}

		playback, err = malgo.InitDevice(malgoCtx.Context, deviceConfig, captureCallbacks)
		if err != nil {
			return nil, nil, err
		}
	}

	return
}

func SelectDevices(ctx *malgo.AllocatedContext) error {
	var err error
	captureDevices, err = ctx.Devices(malgo.Capture)
	if err != nil {
		return err
	}

	fmt.Println("\x1b[34m= Select mic =\x1b[m")
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

	fmt.Println("\x1b[34m= Select Speaker =\x1b[m")
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

	fmt.Printf("\x1b[33mUsing mic %s\x1b[m\n", captureDevices[captureDevIdx].Name())
	fmt.Printf("\x1b[33mUsing speaker %s\x1b[m\n", playbackDevices[playbackDevIdx].Name())

	return nil
}
