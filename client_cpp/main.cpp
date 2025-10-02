#define MINIAUDIO_IMPLEMENTATION
#include "miniaudio.h"

#include <atomic>
#include <cstdio>
#include <cstring>
#include <thread>
#include <chrono>

// Audio configuration matching the Go client
constexpr ma_format FORMAT = ma_format_f32;
constexpr ma_uint32 CHANNELS = 1;
constexpr ma_uint32 SAMPLE_RATE = 48000;

// Simple ring buffer for audio passthrough
constexpr size_t RING_BUFFER_SIZE = SAMPLE_RATE * 2; // 2 seconds buffer
float g_ringBuffer[RING_BUFFER_SIZE];
std::atomic<size_t> g_writePos{0};
std::atomic<size_t> g_readPos{0};
std::atomic<size_t> g_available{0};

// Thread-safe ring buffer operations
void ringBufferWrite(const float* data, size_t frameCount) {
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

size_t ringBufferRead(float* data, size_t frameCount) {
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
void captureCallback(ma_device* pDevice, void* pOutput, const void* pInput, ma_uint32 frameCount) {
    (void)pDevice;
    (void)pOutput;
    
    const float* inputSamples = static_cast<const float*>(pInput);
    
    // Write captured audio to ring buffer
    ringBufferWrite(inputSamples, frameCount * CHANNELS);
    
    // TODO: Add WebRTC APM processing here
    // Example:
    // processedSamples = webrtcAPM->ProcessCaptureStream(inputSamples, frameCount);
    // ringBufferWrite(processedSamples, frameCount * CHANNELS);
}

// Playback callback - writes to speakers
void playbackCallback(ma_device* pDevice, void* pOutput, const void* pInput, ma_uint32 frameCount) {
    (void)pDevice;
    (void)pInput;
    
    float* outputSamples = static_cast<float*>(pOutput);
    
    // Read from ring buffer to playback
    ringBufferRead(outputSamples, frameCount * CHANNELS);
    
    // TODO: Add playback processing here if needed
}

void listDevices(ma_context* pContext, ma_device_type deviceType) {
    ma_device_info* pDeviceInfos;
    ma_uint32 deviceCount;
    
    const char* typeStr = (deviceType == ma_device_type_capture) ? "Capture" : "Playback";
    
    ma_result result = ma_context_get_devices(pContext, &pDeviceInfos, &deviceCount, NULL, NULL);
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

int main() {
    printf("miniaudio Duplex Example\n");
    printf("This example captures audio from your microphone and plays it back.\n\n");
    
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
    ma_device_config captureConfig = ma_device_config_init(ma_device_type_capture);
    captureConfig.capture.format = FORMAT;
    captureConfig.capture.channels = CHANNELS;
    captureConfig.sampleRate = SAMPLE_RATE;
    captureConfig.dataCallback = captureCallback;
    captureConfig.pUserData = nullptr;
    
    // Configure playback device
    ma_device_config playbackConfig = ma_device_config_init(ma_device_type_playback);
    playbackConfig.playback.format = FORMAT;
    playbackConfig.playback.channels = CHANNELS;
    playbackConfig.sampleRate = SAMPLE_RATE;
    playbackConfig.dataCallback = playbackCallback;
    playbackConfig.pUserData = nullptr;
    
    // Initialize devices
    ma_device captureDevice;
    ma_device playbackDevice;
    
    if (ma_device_init(&context, &captureConfig, &captureDevice) != MA_SUCCESS) {
        printf("Failed to initialize capture device.\n");
        ma_context_uninit(&context);
        return -2;
    }
    
    if (ma_device_init(&context, &playbackConfig, &playbackDevice) != MA_SUCCESS) {
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
    
    printf("\n\033[32mAudio streaming active! You should hear your microphone input.\033[0m\n");
    printf("Press Enter to stop...\n");
    
    getchar();
    
    // Cleanup
    printf("\nStopping devices...\n");
    ma_device_uninit(&playbackDevice);
    ma_device_uninit(&captureDevice);
    ma_context_uninit(&context);
    
    printf("Done.\n");
    return 0;
}

