# lekvc C++ Client

A C++ implementation of the lekvc voice client using miniaudio and WebRTC Audio Processing Module.

## Features

- **Duplex audio**: Simultaneous capture and playback
- **miniaudio**: Cross-platform audio library (header-only)
- **CMake build system**: Easy integration with WebRTC APM
- **48kHz mono F32**: Matches the Go client configuration

## Building

### Prerequisites

1. **CMake** (>= 3.15)
2. **C++ compiler** with C++17 support (GCC, Clang, MSVC)
3. **miniaudio.h** - Download it first:

```bash
# Download miniaudio header
wget https://raw.githubusercontent.com/mackron/miniaudio/master/miniaudio.h
```

### Build Steps

```bash
# From client_cpp directory
mkdir build
cd build
cmake ..
make
```

### Run

```bash
./lekvc_client
```

You should hear your microphone input played back through your speakers (be careful of feedback!).

## Project Structure

```
client_cpp/
├── CMakeLists.txt      # Build configuration
├── main.cpp            # Duplex audio example
├── miniaudio.h         # miniaudio library (download separately)
└── README.md           # This file
```

## Adding WebRTC APM

To integrate WebRTC Audio Processing Module later:

1. Install webrtc-audio-processing library:
   ```bash
   # On Ubuntu/Debian
   sudo apt-get install webrtc-audio-processing-dev
   
   # On Arch Linux
   sudo pacman -S webrtc-audio-processing
   ```

2. Uncomment the WebRTC section in `CMakeLists.txt`

3. Add processing in the capture callback in `main.cpp` (marked with TODO comments)

## Configuration

Current audio settings (matching Go client):
- **Sample Rate**: 48000 Hz
- **Channels**: 1 (Mono)
- **Format**: Float32
- **Ring Buffer**: 2 seconds

## Platform Support

- **Linux**: Tested, uses ALSA/PulseAudio
- **macOS**: Should work with CoreAudio
- **Windows**: Should work with WASAPI

## Notes

- The example does a simple audio passthrough from mic to speakers
- Ring buffer prevents audio glitches but adds ~40ms latency
- WebRTC APM integration points are marked with TODO comments
- Compile commands are exported for IDE integration

