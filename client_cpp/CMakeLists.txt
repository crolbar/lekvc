cmake_minimum_required(VERSION 3.15)
project(lekvc_cpp)

set(CMAKE_CXX_STANDARD 17)
set(CMAKE_CXX_STANDARD_REQUIRED ON)
set(CMAKE_EXPORT_COMPILE_COMMANDS ON)

# Prefer static linking for portability
if(WIN32)
    set(CMAKE_FIND_LIBRARY_SUFFIXES .a ${CMAKE_FIND_LIBRARY_SUFFIXES})
    set(BUILD_SHARED_LIBS OFF)
    # Static runtime linking on Windows
    set(CMAKE_MSVC_RUNTIME_LIBRARY "MultiThreaded$<$<CONFIG:Debug>:Debug>")
endif()

# Add executable
add_executable(lekvc_client main.cpp)

# Find WebRTC Audio Processing - different approaches for different platforms
if(WIN32)
    # Windows - try to find the library built from source
    find_package(webrtc-audio-processing CONFIG QUIET)
    if(webrtc-audio-processing_FOUND)
        target_link_libraries(lekvc_client PRIVATE webrtc-audio-processing::webrtc-audio-processing)
        message(STATUS "Found WebRTC Audio Processing via CONFIG")
    else()
        # Try to find it manually
        find_path(WEBRTC_APM_INCLUDE_DIR 
            NAMES modules/audio_processing/include/audio_processing.h
            PATHS ${CMAKE_PREFIX_PATH}/include
            PATH_SUFFIXES webrtc)
        
        find_library(WEBRTC_APM_LIBRARY 
            NAMES webrtc-audio-processing webrtc_audio_processing
            PATHS ${CMAKE_PREFIX_PATH}/lib)
        
        if(WEBRTC_APM_INCLUDE_DIR AND WEBRTC_APM_LIBRARY)
            target_include_directories(lekvc_client PRIVATE ${WEBRTC_APM_INCLUDE_DIR})
            target_link_libraries(lekvc_client PRIVATE ${WEBRTC_APM_LIBRARY})
            message(STATUS "Found WebRTC Audio Processing manually: ${WEBRTC_APM_LIBRARY}")
        else()
            message(FATAL_ERROR "WebRTC Audio Processing not found. Include: ${WEBRTC_APM_INCLUDE_DIR}, Library: ${WEBRTC_APM_LIBRARY}")
        endif()
    endif()
else()
    # Linux/Unix with pkg-config
    find_package(PkgConfig REQUIRED)
    # Try different package names for different distributions
    pkg_check_modules(WEBRTC_APM IMPORTED_TARGET webrtc-audio-processing-1)
    if(NOT WEBRTC_APM_FOUND)
        pkg_check_modules(WEBRTC_APM IMPORTED_TARGET webrtc-audio-processing)
    endif()
    if(NOT WEBRTC_APM_FOUND)
        pkg_check_modules(WEBRTC_APM REQUIRED IMPORTED_TARGET webrtc-audio-processing-2)
    endif()
    target_link_libraries(lekvc_client PRIVATE PkgConfig::WEBRTC_APM)
endif()

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
endif()

# Enable warnings
if(MSVC)
    target_compile_options(lekvc_client PRIVATE /W4)
else()
    target_compile_options(lekvc_client PRIVATE -Wall -Wextra -pedantic)
endif()

# Include current directory for miniaudio.h
target_include_directories(lekvc_client PRIVATE ${CMAKE_CURRENT_SOURCE_DIR})

