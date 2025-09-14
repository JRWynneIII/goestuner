package radio

// #cgo CFLAGS: -g -Wall
// #cgo LDFLAGS: -lSoapySDR
import (
	"github.com/charmbracelet/log"
	"github.com/jrwynneiii/goestuner/config"

	"github.com/pothosware/go-soapy-sdr/pkg/device"
	"github.com/pothosware/go-soapy-sdr/pkg/modules"
	"github.com/pothosware/go-soapy-sdr/pkg/version"
)

type StreamType int

type StreamConstraint interface {
	uint8 | int8 | uint16 | int16 | complex64 | complex128
}

const (
	CU8 StreamType = iota
	CS8
	CU16
	CS16
	CF32
	CF64
)

type Radio[T StreamConstraint] struct {
	Driver     string
	Address    string
	SampleRate float64
	SampleType StreamType
	BufferCU8  [][]uint8
	BufferCS8  [][]int8
	BufferCU16 [][]uint16
	BufferCS16 [][]int16
	BufferCF32 [][]complex64
	BufferCF64 [][]complex128
	Frequency  float64
	//Private:
	args   map[string]string
	device *device.SDRDevice
	stream any
}

func InitSoapySDR() {
	log.Debugf("Using SoapySDR versions: ABI: %s API: %s Lib: %s", version.GetABIVersion(), version.GetAPIVersion(), version.GetLibVersion())
	log.Debugf("SoapySDR modules root path: %v", modules.GetRootPath())

	searchPaths := modules.ListSearchPaths()
	if len(searchPaths) > 0 {
		for i, searchPath := range searchPaths {
			log.Debugf("Search path #%d: %v", i, searchPath)
		}
	} else {
		log.Debug("Search paths: [none]")
	}

	modulesFound := modules.ListModules()
	if len(modulesFound) > 0 {
		for _, module := range modulesFound {
			moduleVersion := modules.GetModuleVersion(module)
			if len(moduleVersion) == 0 {
				moduleVersion = "[None]"
			}
			log.Debugf("Found SoapySDR module: %v, version: %v", module, moduleVersion)
		}
	} else {
		log.Debug("No SoapySDR modules found")
	}
}

func New[T StreamConstraint](conf config.RadioConf, driver string, stype StreamType, bufSize uint) *Radio[T] {
	log.Info("Initing SoapySDR")
	InitSoapySDR()

	r := Radio[T]{
		Driver:     driver,
		SampleRate: conf.SampleRate,
		SampleType: stype,
		Frequency:  conf.Frequency,
		Address:    conf.Address,
	}

	switch stype {
	case CU8:
		r.BufferCU8 = make([][]uint8, 1)
		r.BufferCU8[0] = make([]uint8, bufSize)
	case CS8:
		r.BufferCS8 = make([][]int8, 1)
		r.BufferCS8[0] = make([]int8, bufSize)
	case CU16:
		r.BufferCU16 = make([][]uint16, 1)
		r.BufferCU16[0] = make([]uint16, bufSize)
	case CS16:
		r.BufferCS16 = make([][]int16, 1)
		r.BufferCS16[0] = make([]int16, bufSize)
	case CF32:
		r.BufferCF32 = make([][]complex64, 1)
		r.BufferCF32[0] = make([]complex64, bufSize)
	case CF64:
		r.BufferCF64 = make([][]complex128, 1)
		r.BufferCF64[0] = make([]complex128, bufSize)
	}
	return &r
}

func (r *Radio[T]) Read(num uint) any {
	flags := make([]int, 1)
	timeout := uint(100000) //nanosec

	switch r.SampleType {
	case CU8:
		timeNs, numSamples, err := r.stream.(*device.SDRStreamCU8).Read(r.BufferCU8, num, flags, timeout)
		log.Debugf("timeNs: %v, numSamples: %v, err: %v", timeNs, numSamples, err)
		return r.BufferCU8[0]
	case CS8:
		timeNs, numSamples, err := r.stream.(*device.SDRStreamCS8).Read(r.BufferCS8, num, flags, timeout)
		log.Debugf("timeNs: %v, numSamples: %v, err: %v", timeNs, numSamples, err)
		return r.BufferCS8[0]
	case CU16:
		timeNs, numSamples, err := r.stream.(*device.SDRStreamCU16).Read(r.BufferCU16, num, flags, timeout)
		log.Debugf("timeNs: %v, numSamples: %v, err: %v", timeNs, numSamples, err)
		return r.BufferCU16[0]
	case CS16:
		timeNs, numSamples, err := r.stream.(*device.SDRStreamCS16).Read(r.BufferCS16, num, flags, timeout)
		log.Debugf("timeNs: %v, numSamples: %v, err: %v", timeNs, numSamples, err)
		return r.BufferCS16[0]
	case CF32:
		timeNs, numSamples, err := r.stream.(*device.SDRStreamCF32).Read(r.BufferCF32, num, flags, timeout)
		log.Debugf("timeNs: %v, numSamples: %v, err: %v", timeNs, numSamples, err)
		return r.BufferCF32[0]
	case CF64:
		timeNs, numSamples, err := r.stream.(*device.SDRStreamCF64).Read(r.BufferCF64, num, flags, timeout)
		log.Debugf("timeNs: %v, numSamples: %v, err: %v", timeNs, numSamples, err)
		return r.BufferCF64[0]
	}
	return []T{}
}

func (r *Radio[T]) LogAvailSettings() {
	//Display settings
	log.Debugf("Current settings:")
	settings := r.device.GetSettingInfo()
	if len(settings) > 0 {
		for _, setting := range settings {
			log.Debugf("\t- %s: %v", setting.Key, setting.Value)
		}
	}

	//Get sample rate range
	numChannels := r.device.GetNumChannels(device.DirectionRX)
	log.Debug("Channel info:")
	for channel := uint(0); channel < numChannels; channel++ {
		log.Debugf("Channel %d:", channel)
		log.Debugf("\tAvailable sample rates:")
		log.Debugf("\t\t- %v", r.device.GetSampleRate(device.DirectionRX, channel))
		for _, sampleRateRange := range r.device.GetSampleRateRange(device.DirectionRX, channel) {
			log.Debugf("\t\t- %v", sampleRateRange.ToString())
		}
		log.Debugf("\tIQ Sample Types: %v", r.device.GetStreamFormats(device.DirectionRX, channel))
	}
}

func (r *Radio[T]) Connect() {
	r.args = make(map[string]string)
	r.args["driver"] = r.Driver
	if r.Driver == "rtltcp" {
		r.args["rtltcp"] = r.Address
	}
	// Create the soapysdr device object
	var err error
	if r.device, err = device.Make(r.args); err != nil {
		log.Fatalf("Could not create SoapySDR device! %s", err.Error())
	}

	//Set the sample rate
	log.Debugf("Setting sample rate to %f", r.SampleRate)
	if err := r.device.SetSampleRate(device.DirectionRX, 0, r.SampleRate); err != nil {
		log.Fatalf("Could not set sample rate! %s", err.Error())
	}

	//set the frequency
	log.Debugf("Setting frequency to %f", r.Frequency)
	if err := r.device.SetFrequency(device.DirectionRX, 0, r.Frequency, nil); err != nil {
		log.Fatalf("Could not set frequency! %s", err.Error())
	}

	log.Infof("Initialized device: %v", r.Driver)

	if r.Driver != "rtltcp" {
		r.LogAvailSettings()
	}

	//Create the IQ stream
	log.Info("Creating the IQ stream")
	switch r.SampleType {
	case CU8:
		if r.stream, err = r.device.SetupSDRStreamCU8(device.DirectionRX, []uint{0}, nil); err != nil {
			log.Fatalf("Could not setup SDR stream! %s", err.Error())
		}
	case CS8:
		if r.stream, err = r.device.SetupSDRStreamCS8(device.DirectionRX, []uint{0}, nil); err != nil {
			log.Fatalf("Could not setup SDR stream! %s", err.Error())
		}
	case CU16:
		if r.stream, err = r.device.SetupSDRStreamCU16(device.DirectionRX, []uint{0}, nil); err != nil {
			log.Fatalf("Could not setup SDR stream! %s", err.Error())
		}
	case CS16:
		if r.stream, err = r.device.SetupSDRStreamCS16(device.DirectionRX, []uint{0}, nil); err != nil {
			log.Fatalf("Could not setup SDR stream! %s", err.Error())
		}
	case CF32:
		if r.stream, err = r.device.SetupSDRStreamCF32(device.DirectionRX, []uint{0}, nil); err != nil {
			log.Fatalf("Could not setup SDR stream! %s", err.Error())
		}
	case CF64:
		if r.stream, err = r.device.SetupSDRStreamCF64(device.DirectionRX, []uint{0}, nil); err != nil {
			log.Fatalf("Could not setup SDR stream! %s", err.Error())
		}
	}

	//Activate the stream
	r.StreamActivate()
}

func (r *Radio[T]) StreamActivate() {
	log.Debug("Activating IQ stream")
	switch r.SampleType {
	case CU8:
		log.Info("Activating IQ stream...")
		if err := r.stream.(*device.SDRStreamCU8).Activate(0, 0, 0); err != nil {
			log.Fatalf("Could not activate the IQ stream! %s", err.Error())
		}
	case CS8:
		log.Info("Activating IQ stream...")
		if err := r.stream.(*device.SDRStreamCS8).Activate(0, 0, 0); err != nil {
			log.Fatalf("Could not activate the IQ stream! %s", err.Error())
		}
	case CU16:
		log.Info("Activating IQ stream...")
		if err := r.stream.(*device.SDRStreamCU16).Activate(0, 0, 0); err != nil {
			log.Fatalf("Could not activate the IQ stream! %s", err.Error())
		}
	case CS16:
		log.Info("Activating IQ stream...")
		if err := r.stream.(*device.SDRStreamCS16).Activate(0, 0, 0); err != nil {
			log.Fatalf("Could not activate the IQ stream! %s", err.Error())
		}
	case CF32:
		log.Info("Activating IQ stream...")
		if err := r.stream.(*device.SDRStreamCF32).Activate(0, 0, 0); err != nil {
			log.Fatalf("Could not activate the IQ stream! %s", err.Error())
		}
	case CF64:
		log.Info("Activating IQ stream...")
		if err := r.stream.(*device.SDRStreamCF64).Activate(0, 0, 0); err != nil {
			log.Fatalf("Could not activate the IQ stream! %s", err.Error())
		}
	}
	//Read the first few samples and discard to make sure we have clean data
	r.Read(1024)
	if len(r.BufferCU8) > 0 {
		clear(r.BufferCU8[0])
	}
	if len(r.BufferCS8) > 0 {
		clear(r.BufferCS8[0])
	}
	if len(r.BufferCU16) > 0 {
		clear(r.BufferCU16[0])
	}
	if len(r.BufferCS16) > 0 {
		clear(r.BufferCS16[0])
	}
	if len(r.BufferCF32) > 0 {
		clear(r.BufferCF32[0])
	}
	if len(r.BufferCF64) > 0 {
		clear(r.BufferCF64[0])
	}
}

func (r *Radio[T]) StreamDeactivate() {
	log.Info("Deactivating IQ stream...")
	if r.stream != nil {
		switch r.SampleType {
		case CU8:
			if err := r.stream.(*device.SDRStreamCU8).Deactivate(0, 0); err != nil {
				log.Fatalf("Could not deactivate the IQ stream! %s", err.Error())
			}
		case CS8:
			if err := r.stream.(*device.SDRStreamCS8).Deactivate(0, 0); err != nil {
				log.Fatalf("Could not deactivate the IQ stream! %s", err.Error())
			}
		case CU16:
			if err := r.stream.(*device.SDRStreamCU16).Deactivate(0, 0); err != nil {
				log.Fatalf("Could not deactivate the IQ stream! %s", err.Error())
			}
		case CS16:
			if err := r.stream.(*device.SDRStreamCS16).Deactivate(0, 0); err != nil {
				log.Fatalf("Could not deactivate the IQ stream! %s", err.Error())
			}
		case CF32:
			if err := r.stream.(*device.SDRStreamCF32).Deactivate(0, 0); err != nil {
				log.Fatalf("Could not deactivate the IQ stream! %s", err.Error())
			}
		case CF64:
			if err := r.stream.(*device.SDRStreamCF64).Deactivate(0, 0); err != nil {
				log.Fatalf("Could not deactivate the IQ stream! %s", err.Error())
			}
		}
	}
}

func (r *Radio[T]) StreamClose() {
	log.Info("Closing IQ stream...")
	if r.stream != nil {
		switch r.SampleType {
		case CU8:
			if err := r.stream.(*device.SDRStreamCU8).Close(); err != nil {
				log.Fatalf("Could not close the IQ stream! %s", err.Error())
			}
		case CS8:
			if err := r.stream.(*device.SDRStreamCS8).Close(); err != nil {
				log.Fatalf("Could not close the IQ stream! %s", err.Error())
			}
		case CU16:
			if err := r.stream.(*device.SDRStreamCU16).Close(); err != nil {
				log.Fatalf("Could not close the IQ stream! %s", err.Error())
			}
		case CS16:
			if err := r.stream.(*device.SDRStreamCS16).Close(); err != nil {
				log.Fatalf("Could not close the IQ stream! %s", err.Error())
			}
		case CF32:
			if err := r.stream.(*device.SDRStreamCF32).Close(); err != nil {
				log.Fatalf("Could not close the IQ stream! %s", err.Error())
			}
		case CF64:
			if err := r.stream.(*device.SDRStreamCF64).Close(); err != nil {
				log.Fatalf("Could not close the IQ stream! %s", err.Error())
			}
		}
	}
}

func (r *Radio[T]) Destroy() {
	r.StreamDeactivate()
	r.StreamClose()
}
