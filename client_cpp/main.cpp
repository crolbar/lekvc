#define MINIAUDIO_IMPLEMENTATION
#include "miniaudio.h"

#include <modules/audio_processing/include/audio_processing.h>

#include <algorithm>
#include <atomic>
#include <chrono>
#include <cstdio>
#include <cstring>
#include <memory>
#include <mutex>
#include <thread>
#include <vector>

// Audio configuration matching the Go client
constexpr ma_format FORMAT = ma_format_f32;
constexpr ma_uint32 CHANNELS = 1;
constexpr ma_uint32 SAMPLE_RATE = 48000;

// WebRTC APM works with 10ms frames at 48kHz = 480 samples
constexpr size_t APM_FRAME_SIZE = 480; // 10ms at 48kHz

// Simple ring buffer for audio passthrough
constexpr size_t RING_BUFFER_SIZE = SAMPLE_RATE * 2; // 2 seconds buffer
float g_ringBuffer[RING_BUFFER_SIZE];
std::atomic<size_t> g_writePos{ 0 };
std::atomic<size_t> g_readPos{ 0 };
std::atomic<size_t> g_available{ 0 };

// WebRTC APM instance (v2 uses scoped_refptr)
rtc::scoped_refptr<webrtc::AudioProcessing> g_apm;

// Frame accumulator for WebRTC APM (needs exactly 10ms frames)
std::vector<float> g_frameAccumulator;
std::mutex g_accumulatorMutex;

// Initialize WebRTC APM
bool
initializeWebRTCApm()
{
    webrtc::AudioProcessing::Config config;

    // Enable noise suppression
    config.noise_suppression.enabled = true;
    config.noise_suppression.level =
      webrtc::AudioProcessing::Config::NoiseSuppression::kHigh;

    // Disable other features for now (you can enable them later)
    config.echo_canceller.enabled = false;
    config.gain_controller1.enabled = false;
    config.gain_controller2.enabled = false;
    config.high_pass_filter.enabled = true; // Usually good to keep enabled

    // Create APM instance
    g_apm = webrtc::AudioProcessingBuilder().Create();
    if (!g_apm) {
        printf("Failed to create WebRTC APM instance\n");
        return false;
    }

    // Apply configuration
    g_apm->ApplyConfig(config);

    // Set stream formats
    webrtc::StreamConfig stream_config(SAMPLE_RATE, CHANNELS);

    printf("WebRTC APM initialized successfully\n");
    printf("  Noise Suppression: High\n");
    printf("  High-pass filter: Enabled\n");

    return true;
}

// Convert float32 [-1.0, 1.0] to int16 [-32768, 32767]
void
floatToInt16(const float* src, int16_t* dst, size_t count)
{
    for (size_t i = 0; i < count; i++) {
        float sample = src[i];
        // Clamp to [-1.0, 1.0]
        sample = std::max(-1.0f, std::min(1.0f, sample));
        dst[i] = static_cast<int16_t>(sample * 32767.0f);
    }
}

// Convert int16 [-32768, 32767] to float32 [-1.0, 1.0]
void
int16ToFloat(const int16_t* src, float* dst, size_t count)
{
    for (size_t i = 0; i < count; i++) {
        dst[i] = static_cast<float>(src[i]) / 32768.0f;
    }
}

// Process audio through WebRTC APM
void
processWithAPM(const float* input, float* output, size_t frameCount)
{
    if (!g_apm || frameCount != APM_FRAME_SIZE) {
        // If APM not available or wrong frame size, just copy
        memcpy(output, input, frameCount * sizeof(float));
        return;
    }

    // Convert float32 to int16 (WebRTC APM native format)
    std::vector<int16_t> int16Input(frameCount);
    std::vector<int16_t> int16Output(frameCount);

    floatToInt16(input, int16Input.data(), frameCount);

    // Process the audio (WebRTC APM v2 API)
    webrtc::StreamConfig stream_config(SAMPLE_RATE, CHANNELS);

    int result = g_apm->ProcessStream(int16Input.data(), // src
                                      stream_config,     // input_config
                                      stream_config,     // output_config
                                      int16Output.data() // dest
    );

    if (result != 0) {
        printf("APM ProcessStream error: %d\n", result);
        memcpy(output, input, frameCount * sizeof(float));
        return;
    }

    // Convert back to float32
    int16ToFloat(int16Output.data(), output, frameCount);
}

// Thread-safe ring buffer operations
void
ringBufferWrite(const float* data, size_t frameCount)
{
    size_t writePos = g_writePos.load(std::memory_order_relaxed);
    size_t available = g_available.load(std::memory_order_acquire);

    for (size_t i = 0; i < frameCount; i++) {
        if (available < RING_BUFFER_SIZE) {
            g_ringBuffer[writePos] = data[i];
            writePos = (writePos + 1) % RING_BUFFER_SIZE;
            available++;
        }
    }

    g_writePos.store(writePos, std::memory_order_relaxed);
    g_available.store(available, std::memory_order_release);
}

size_t
ringBufferRead(float* data, size_t frameCount)
{
    size_t readPos = g_readPos.load(std::memory_order_relaxed);
    size_t available = g_available.load(std::memory_order_acquire);

    size_t framesToRead = (frameCount < available) ? frameCount : available;

    for (size_t i = 0; i < framesToRead; i++) {
        data[i] = g_ringBuffer[readPos];
        readPos = (readPos + 1) % RING_BUFFER_SIZE;
    }

    g_readPos.store(readPos, std::memory_order_relaxed);
    g_available.store(available - framesToRead, std::memory_order_release);

    // Fill remaining with silence if we didn't have enough data
    for (size_t i = framesToRead; i < frameCount; i++) {
        data[i] = 0.0f;
    }

    return framesToRead;
}

// Capture callback - reads from microphone
void
captureCallback(ma_device* pDevice,
                void* pOutput,
                const void* pInput,
                ma_uint32 frameCount)
{
    // (void)pDevice;
    // (void)pOutput;

    // const float* inputSamples = static_cast<const float*>(pInput);

    // // Write captured audio to ring buffer
    // ringBufferWrite(inputSamples, frameCount * CHANNELS);

    // return;

    (void)pDevice;
    (void)pOutput;

    const float* inputSamples = static_cast<const float*>(pInput);
    size_t totalSamples = frameCount * CHANNELS;

    // Lock the accumulator
    std::lock_guard<std::mutex> lock(g_accumulatorMutex);

    // Add incoming samples to accumulator
    g_frameAccumulator.insert(
      g_frameAccumulator.end(), inputSamples, inputSamples + totalSamples);

    // Process all complete APM frames
    while (g_frameAccumulator.size() >= APM_FRAME_SIZE) {
        // Extract one APM frame
        std::vector<float> frame(g_frameAccumulator.begin(),
                                 g_frameAccumulator.begin() + APM_FRAME_SIZE);

        // Process through WebRTC APM
        std::vector<float> processedFrame(APM_FRAME_SIZE);
        processWithAPM(frame.data(), processedFrame.data(), APM_FRAME_SIZE);

        // Write processed audio to ring buffer
        ringBufferWrite(processedFrame.data(), APM_FRAME_SIZE);

        // Remove processed samples from accumulator
        g_frameAccumulator.erase(g_frameAccumulator.begin(),
                                 g_frameAccumulator.begin() + APM_FRAME_SIZE);
    }

    // Prevent accumulator from growing too large
    if (g_frameAccumulator.size() > APM_FRAME_SIZE * 2) {
        g_frameAccumulator.erase(
          g_frameAccumulator.begin(),
          g_frameAccumulator.begin() +
            (g_frameAccumulator.size() - APM_FRAME_SIZE));
    }
}

// Playback callback - writes to speakers
void
playbackCallback(ma_device* pDevice,
                 void* pOutput,
                 const void* pInput,
                 ma_uint32 frameCount)
{
    (void)pDevice;
    (void)pInput;

    float* outputSamples = static_cast<float*>(pOutput);

    // Read from ring buffer to playback
    ringBufferRead(outputSamples, frameCount * CHANNELS);

    // TODO: Add playback processing here if needed
}

void
listDevices(ma_context* pContext, ma_device_type deviceType)
{
    ma_device_info* pDeviceInfos;
    ma_uint32 deviceCount;

    const char* typeStr =
      (deviceType == ma_device_type_capture) ? "Capture" : "Playback";

    ma_result result =
      ma_context_get_devices(pContext, &pDeviceInfos, &deviceCount, NULL, NULL);
    if (result != MA_SUCCESS) {
        printf("Failed to get %s devices.\n", typeStr);
        return;
    }

    printf("\n=== %s Devices ===\n", typeStr);
    for (ma_uint32 i = 0; i < deviceCount; i++) {
        if (pDeviceInfos[i].isDefault) {
            printf("[%u] %s (DEFAULT)\n", i, pDeviceInfos[i].name);
        } else {
            printf("[%u] %s\n", i, pDeviceInfos[i].name);
        }
    }
}

int
main()
{
    printf("miniaudio + WebRTC APM Example\n");
    printf("This example captures audio with noise suppression and plays it "
           "back.\n\n");

    // Initialize WebRTC APM
    if (!initializeWebRTCApm()) {
        printf("Failed to initialize WebRTC APM.\n");
        return -1;
    }

    // Initialize accumulator
    g_frameAccumulator.reserve(APM_FRAME_SIZE * 2);

    // Initialize context
    ma_context context;
    if (ma_context_init(NULL, 0, NULL, &context) != MA_SUCCESS) {
        printf("Failed to initialize context.\n");
        return -1;
    }

    // List available devices
    listDevices(&context, ma_device_type_capture);
    listDevices(&context, ma_device_type_playback);

    // Configure capture device
    ma_device_config captureConfig =
      ma_device_config_init(ma_device_type_capture);
    captureConfig.capture.format = FORMAT;
    captureConfig.capture.channels = CHANNELS;
    captureConfig.sampleRate = SAMPLE_RATE;
    captureConfig.dataCallback = captureCallback;
    captureConfig.pUserData = nullptr;

    // Configure playback device
    ma_device_config playbackConfig =
      ma_device_config_init(ma_device_type_playback);
    playbackConfig.playback.format = FORMAT;
    playbackConfig.playback.channels = CHANNELS;
    playbackConfig.sampleRate = SAMPLE_RATE;
    playbackConfig.dataCallback = playbackCallback;
    playbackConfig.pUserData = nullptr;

    // Initialize devices
    ma_device captureDevice;
    ma_device playbackDevice;

    if (ma_device_init(&context, &captureConfig, &captureDevice) !=
        MA_SUCCESS) {
        printf("Failed to initialize capture device.\n");
        ma_context_uninit(&context);
        return -2;
    }

    if (ma_device_init(&context, &playbackConfig, &playbackDevice) !=
        MA_SUCCESS) {
        printf("Failed to initialize playback device.\n");
        ma_device_uninit(&captureDevice);
        ma_context_uninit(&context);
        return -3;
    }

    printf("\nUsing capture device: %s\n", captureDevice.capture.name);
    printf("Using playback device: %s\n", playbackDevice.playback.name);
    printf("\nConfiguration:\n");
    printf("  Format: F32\n");
    printf("  Channels: %u\n", CHANNELS);
    printf("  Sample Rate: %u Hz\n", SAMPLE_RATE);

    // Start devices
    if (ma_device_start(&captureDevice) != MA_SUCCESS) {
        printf("Failed to start capture device.\n");
        ma_device_uninit(&playbackDevice);
        ma_device_uninit(&captureDevice);
        ma_context_uninit(&context);
        return -4;
    }

    if (ma_device_start(&playbackDevice) != MA_SUCCESS) {
        printf("Failed to start playback device.\n");
        ma_device_uninit(&playbackDevice);
        ma_device_uninit(&captureDevice);
        ma_context_uninit(&context);
        return -5;
    }

    printf("\n\033[32mAudio streaming active with noise suppression!\033[0m\n");
    printf(
      "You should hear your microphone input with background noise reduced.\n");
    printf("Press Enter to stop...\n");

    getchar();

    // Cleanup
    printf("\nStopping devices...\n");
    ma_device_uninit(&playbackDevice);
    ma_device_uninit(&captureDevice);
    ma_context_uninit(&context);

    // Clean up APM (scoped_refptr auto-releases)
    g_apm = nullptr;

    printf("Done.\n");
    return 0;
}
