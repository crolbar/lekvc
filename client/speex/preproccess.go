package speex

/*
#cgo pkg-config: speexdsp
#include <speex/speex_preprocess.h>
#include <stdlib.h>
*/
import "C"
import (
	"errors"
	"fmt"
	"unsafe"
)

// SpeexPreprocessor handles noise suppression, AGC, VAD, and dereverb
type SpeexPreprocessor struct {
	state      *C.SpeexPreprocessState
	frameSize  int
	sampleRate int
}

// NewSpeexPreprocessor creates a new preprocessor instance
func NewSpeexPreprocessor(frameSize, sampleRate int) (*SpeexPreprocessor, error) {
	state := C.speex_preprocess_state_init(C.int(frameSize), C.int(sampleRate))
	if state == nil {
		return nil, fmt.Errorf("failed to initialize speex preprocessor")
	}

	sp := &SpeexPreprocessor{
		state:      state,
		frameSize:  frameSize,
		sampleRate: sampleRate,
	}

	sp.SetNoiseSuppress(-25)
	sp.SetDenoise(true)
	sp.SetDereverb(true)

	return sp, nil
}

func (ps *SpeexPreprocessor) Run(x []int16) error {
	if ps.state == nil {
		return errors.New("ps state is nil")
	}

	if len(x) == 0 {
		return errors.New("samples buf in preprocessor is empty")
	}

	C.speex_preprocess_run(ps.state, (*C.spx_int16_t)(unsafe.Pointer(&x[0])))
	return nil
}

// Destroy frees the preprocessor resources
func (sp *SpeexPreprocessor) Destroy() {
	if sp.state != nil {
		C.speex_preprocess_state_destroy(sp.state)
		sp.state = nil
	}
}

func (ps *SpeexPreprocessor) Control(request int, ptr unsafe.Pointer) error {
	if ps.state == nil {
		return errors.New("invalid preprocess state")
	}

	ret := C.speex_preprocess_ctl(ps.state, C.int(request), ptr)
	if ret != 0 {
		return errors.New("speex_preprocess_ctl failed")
	}
	return nil
}

func (ps *SpeexPreprocessor) SetDenoise(enabled bool) error {
	val := 0
	if enabled {
		val = 1
	}
	return ps.Control(C.SPEEX_PREPROCESS_SET_DENOISE, unsafe.Pointer(&val))
}

func (ps *SpeexPreprocessor) GetDenoise() (bool, error) {
	var val C.int
	err := ps.Control(C.SPEEX_PREPROCESS_GET_DENOISE, unsafe.Pointer(&val))
	return val == 1, err
}

func (ps *SpeexPreprocessor) SetDereverb(enabled bool) error {
	val := 0
	if enabled {
		val = 1
	}
	return ps.Control(C.SPEEX_PREPROCESS_SET_DEREVERB, unsafe.Pointer(&val))
}

func (ps *SpeexPreprocessor) SetNoiseSuppress(value int) error {
	val := C.int(value)
	return ps.Control(C.SPEEX_PREPROCESS_SET_NOISE_SUPPRESS, unsafe.Pointer(&val))
}

func (ps *SpeexPreprocessor) GetNoiseSuppress() (int, error) {
	var val C.int
	err := ps.Control(C.SPEEX_PREPROCESS_GET_NOISE_SUPPRESS, unsafe.Pointer(&val))
	return int(val), err
}

func (ps *SpeexPreprocessor) SetEchoSuppress(value int) error {
	val := C.int(value)
	return ps.Control(C.SPEEX_PREPROCESS_SET_ECHO_SUPPRESS, unsafe.Pointer(&val))
}

func (ps *SpeexPreprocessor) GetEchoSuppress() (int, error) {
	var val C.int
	err := ps.Control(C.SPEEX_PREPROCESS_GET_ECHO_SUPPRESS, unsafe.Pointer(&val))
	return int(val), err
}

func (ps *SpeexPreprocessor) SetEchoSuppressActive(value int) error {
	val := C.int(value)
	return ps.Control(C.SPEEX_PREPROCESS_SET_ECHO_SUPPRESS_ACTIVE, unsafe.Pointer(&val))
}

func (ps *SpeexPreprocessor) GetEchoSuppressActive() (int, error) {
	var val C.int
	err := ps.Control(C.SPEEX_PREPROCESS_GET_ECHO_SUPPRESS_ACTIVE, unsafe.Pointer(&val))
	return int(val), err
}

// func (ps *SpeexPreprocessor) SetEchoState(state *C.SpeexEchoState) error {
// 	return ps.Control(C.SPEEX_PREPROCESS_SET_ECHO_STATE, unsafe.Pointer(state))
// }

func (ps *SpeexPreprocessor) GetEchoState() (int, error) {
	var val C.int
	err := ps.Control(C.SPEEX_PREPROCESS_GET_ECHO_STATE, unsafe.Pointer(&val))
	return int(val), err
}
