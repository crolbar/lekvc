cmake_minimum_required(VERSION 3.15)
project(lekvc_cpp)

set(CMAKE_CXX_STANDARD 17)
set(CMAKE_CXX_STANDARD_REQUIRED ON)
set(CMAKE_EXPORT_COMPILE_COMMANDS ON)

# Add executable
add_executable(lekvc_client main.cpp)

# Platform-specific libraries for miniaudio
if(UNIX AND NOT APPLE)
    # Linux
    target_link_libraries(lekvc_client PRIVATE pthread m dl)
elseif(APPLE)
    # macOS
    find_library(COREAUDIO_LIBRARY CoreAudio)
    find_library(COREFOUNDATION_LIBRARY CoreFoundation)
    target_link_libraries(lekvc_client PRIVATE ${COREAUDIO_LIBRARY} ${COREFOUNDATION_LIBRARY})
elseif(WIN32)
    # Windows
    target_link_libraries(lekvc_client PRIVATE winmm ole32)
endif()

# Enable warnings
if(MSVC)
    target_compile_options(lekvc_client PRIVATE /W4)
else()
    target_compile_options(lekvc_client PRIVATE -Wall -Wextra -pedantic)
endif()

# Include current directory for miniaudio.h
target_include_directories(lekvc_client PRIVATE ${CMAKE_CURRENT_SOURCE_DIR})

# Future: Add WebRTC APM integration here
# Example:
# find_package(webrtc-audio-processing REQUIRED)
# target_link_libraries(lekvc_client PRIVATE webrtc-audio-processing)

