cmake_minimum_required(VERSION 3.15)
project(lekvc_cpp)

set(CMAKE_CXX_STANDARD 17)
set(CMAKE_CXX_STANDARD_REQUIRED ON)
set(CMAKE_EXPORT_COMPILE_COMMANDS ON)

# Prefer static linking for portability
if(WIN32)
    set(CMAKE_FIND_LIBRARY_SUFFIXES .a ${CMAKE_FIND_LIBRARY_SUFFIXES})
    set(BUILD_SHARED_LIBS OFF)
endif()

# Add executable
add_executable(lekvc_client main.cpp)

# Find WebRTC Audio Processing
find_package(PkgConfig REQUIRED)
pkg_check_modules(WEBRTC_APM REQUIRED IMPORTED_TARGET webrtc-audio-processing-2)

# Link WebRTC APM
target_link_libraries(lekvc_client PRIVATE PkgConfig::WEBRTC_APM)

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
    # Windows - static linking preferred
    target_link_libraries(lekvc_client PRIVATE winmm ole32)
    # Static runtime on Windows
    set_property(TARGET lekvc_client PROPERTY MSVC_RUNTIME_LIBRARY "MultiThreaded$<$<CONFIG:Debug>:Debug>")
endif()

# Enable warnings
if(MSVC)
    target_compile_options(lekvc_client PRIVATE /W4)
else()
    target_compile_options(lekvc_client PRIVATE -Wall -Wextra -pedantic)
endif()

# Include current directory for miniaudio.h
target_include_directories(lekvc_client PRIVATE ${CMAKE_CURRENT_SOURCE_DIR})

