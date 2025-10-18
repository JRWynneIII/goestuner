package radio

// #cgo CFLAGS: -g -Wall
// #cgo LDFLAGS: -lSoapySDR
import (
	"fmt"
	"time"

	"github.com/charmbracelet/log"
	"github.com/jrwynneiii/goestuner/types"

	"github.com/pothosware/go-soapy-sdr/pkg/device"
	"github.com/pothosware/go-soapy-sdr/pkg/modules"
	"github.com/pothosware/go-soapy-sdr/pkg/sdrlogger"
	"github.com/pothosware/go-soapy-sdr/pkg/version"
)

type Radio struct {
	SamplesOutput *chan []complex64
	Driver        string
	Address       string
	SampleRate    float64
	BufferCF32    [][]complex64
	Frequency     float64
	//Private:
	chunksize uint
	args      map[string]string
	device    *device.SDRDevice
	stream    any
	Stopping  bool
}

// Tracks overall information about the SoapySDR environment and available devices/modules
type SoapySubsystem struct {
	ABIVersion string
	APIVersion string
	LibVersion string
	Modules    []SoapyModule
	Devices    []*device.SDRDevice
}

type SoapyModule struct {
	Name    string
	Version string
}

func (s *SoapySubsystem) PrintVersion(debug bool) {
	if debug {
		log.Debugf("Using SoapySDR versions: ABI: %s API: %s Lib: %s", s.ABIVersion, s.APIVersion, s.LibVersion)
	} else {
		log.Infof("Using SoapySDR versions: ABI: %s API: %s Lib: %s", s.ABIVersion, s.APIVersion, s.LibVersion)
	}
}

func (s *SoapySubsystem) PrintModuleInfo(debug bool) {
	for _, m := range s.Modules {
		if debug {
			log.Debugf("Found SoapySDR module: %v, version: %v", m.Name, m.Version)
		} else {
			log.Infof("Found SoapySDR module: %v, version: %v", m.Name, m.Version)
		}
	}
}

func (s *SoapySubsystem) PrintSearchPaths(debug bool) {
	searchPaths := modules.ListSearchPaths()
	if len(searchPaths) > 0 {
		for i, searchPath := range searchPaths {
			if debug {
				log.Debugf("Search path #%d: %v", i, searchPath)
			} else {
				log.Infof("Search path #%d: %v", i, searchPath)
			}
		}
	} else {
		if debug {
			log.Debug("Search paths: [none]")
		} else {
			log.Info("Search paths: [none]")
		}
	}
}

func InitSoapySDR() (*SoapySubsystem, error) {
	subsys := SoapySubsystem{
		ABIVersion: version.GetABIVersion(),
		APIVersion: version.GetAPIVersion(),
		LibVersion: version.GetLibVersion(),
	}

	subsys.PrintVersion(true)

	log.Debugf("SoapySDR modules root path: %v", modules.GetRootPath())

	subsys.PrintSearchPaths(true)

	modulesFound := modules.ListModules()
	if len(modulesFound) > 0 {
		for _, module := range modulesFound {
			moduleVersion := modules.GetModuleVersion(module)
			if len(moduleVersion) == 0 {
				moduleVersion = "[None]"
			}
			subsys.Modules = append(subsys.Modules, SoapyModule{module, moduleVersion})
		}
	} else {
		return &subsys, fmt.Errorf("No SoapySDR modules found")
	}

	subsys.PrintModuleInfo(true)
	sdrlogger.SetLogLevel(sdrlogger.Error)
	return &subsys, nil
}

func LogAllSoapySDRDevices() {
	// List Soapy library information
	log.Infof("Using SoapySDR versions: ABI: %s API: %s Lib: %s", version.GetABIVersion(), version.GetAPIVersion(), version.GetLibVersion())
	log.Infof("SoapySDR modules root path: %v", modules.GetRootPath())

	modulesFound := modules.ListModules()
	if len(modulesFound) > 0 {
		for _, module := range modulesFound {
			moduleVersion := modules.GetModuleVersion(module)
			if len(moduleVersion) == 0 {
				moduleVersion = "[None]"
			}
			log.Infof("Found SoapySDR module: %v, version: %v", module, moduleVersion)
		}
	} else {
		log.Info("No SoapySDR modules found")
	}

	// Tune down the logger for soapy so that it doesn't yell about rtl-tcp
	sdrlogger.SetLogLevel(sdrlogger.Error)

	// Find all our devices and list info
	devices := device.Enumerate(nil)
	log.Infof("Found %d devices", len(devices))
	args := make([]map[string]string, len(devices))
	for idx, dev := range devices {
		args[idx] = map[string]string{"driver": dev["driver"]}
	}
	if devs, err := device.MakeList(args); err == nil {
		for idx, dev := range devs {
			log.Infof("Driver: %s", args[idx]["driver"])
			LogAvailSettings(dev)
		}
		// This appears to do a double free in the cgo library for Soapy. Idfk why so we're
		// Not being a good dev here and letting the OS close the device i guess
		//if err := device.UnmakeList(devs); err != nil {
		//	log.Errorf("Could not close SDR devices: %v", err)
		//}
	} else {
		log.Fatalf("SoapySDR could not open devices: %v", err)
	}

}

func (r *Radio) Start() {
	var buf []complex64
	for {
		if !r.Stopping {
			samples := r.Read(r.chunksize)
			buf = append(buf, samples.([]complex64)...)

			if len(buf) >= int(r.chunksize) {
				*r.SamplesOutput <- buf
				buf = []complex64{}
			}
		}
		time.Sleep(5 * time.Millisecond)
	}

}

func New(conf types.RadioConf, driver string, bufSize uint, output *chan []complex64) *Radio {
	log.Debug("Initing SoapySDR")
	if _, err := InitSoapySDR(); err != nil {
		panic(err)
	}

	r := Radio{
		Driver:        driver,
		SampleRate:    conf.SampleRate,
		Frequency:     conf.Frequency,
		Address:       conf.Address,
		SamplesOutput: output,
		chunksize:     bufSize,
	}

	r.BufferCF32 = make([][]complex64, 1)
	r.BufferCF32[0] = make([]complex64, bufSize)

	return &r
}

func (r *Radio) Pause() {
	r.Stopping = true
	r.StreamDeactivate()
	r.StreamClose()
	r.BufferCF32 = make([][]complex64, 1)
	r.BufferCF32[0] = make([]complex64, r.chunksize)
}

func (r *Radio) Read(num uint) any {
	flags := make([]int, 1)
	timeout := uint(100000) //nanosec

	if !r.Stopping {
		timeNs, numSamples, err := r.stream.(*device.SDRStreamCF32).Read(r.BufferCF32, num, flags, timeout)
		log.Debugf("timeNs: %v, numSamples: %v, err: %v", timeNs, numSamples, err)
		return r.BufferCF32[0][:numSamples]
	}

	return []complex64{}
}

func LogAvailSettings(dev *device.SDRDevice) {
	//Display settings
	log.Infof("Current settings:")
	settings := dev.GetSettingInfo()
	if len(settings) > 0 {
		for _, setting := range settings {
			log.Infof("\t- %s: %v", setting.Key, setting.Value)
		}
	}

	//Get sample rate range
	numChannels := dev.GetNumChannels(device.DirectionRX)
	log.Info("Channel info:")
	for channel := uint(0); channel < numChannels; channel++ {
		log.Infof("Channel %d:", channel)
		log.Infof("\tAvailable sample rates:")
		log.Infof("\t\t- %v", dev.GetSampleRate(device.DirectionRX, channel))
		for _, sampleRateRange := range dev.GetSampleRateRange(device.DirectionRX, channel) {
			log.Infof("\t\t- %v", sampleRateRange.ToString())
		}
		log.Infof("\tIQ Sample Types: %v", dev.GetStreamFormats(device.DirectionRX, channel))
	}
}

func (r *Radio) Connect() {
	r.args = make(map[string]string)
	r.args["driver"] = r.Driver
	if r.Driver == "rtltcp" {
		r.args["rtltcp"] = r.Address
	}
	// Create the soapysdr device object
	var err error
	if r.device == nil {
		if r.device, err = device.Make(r.args); err != nil {
			log.Fatalf("Could not create SoapySDR device! %s", err.Error())
		}
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

	log.Debugf("Initialized device: %v", r.Driver)

	if r.Driver != "rtltcp" {
		LogAvailSettings(r.device)
	}

	//Create the IQ stream
	log.Debug("Creating the IQ stream")
	if r.stream, err = r.device.SetupSDRStreamCF32(device.DirectionRX, []uint{0}, nil); err != nil {
		log.Fatalf("Could not setup SDR stream! %s", err.Error())
	}

	//Activate the stream
	r.StreamActivate()
}

func (r *Radio) StreamActivate() {
	log.Debug("Activating IQ stream")
	log.Debug("Activating IQ stream...")
	if err := r.stream.(*device.SDRStreamCF32).Activate(0, 0, 0); err != nil {
		log.Fatalf("Could not activate the IQ stream! %s", err.Error())
	}
	//Read the first few samples and discard to make sure we have clean data
	r.Stopping = false
	r.Read(1024)
	if len(r.BufferCF32) > 0 {
		clear(r.BufferCF32[0])
	}
}

func (r *Radio) StreamDeactivate() {
	log.Debug("Deactivating IQ stream...")
	if r.stream != nil {
		if err := r.stream.(*device.SDRStreamCF32).Deactivate(0, 0); err != nil {
			log.Fatalf("Could not deactivate the IQ stream! %s", err.Error())
		}
	}
}

func (r *Radio) StreamClose() {
	log.Debug("Closing IQ stream...")
	if r.stream != nil {
		if err := r.stream.(*device.SDRStreamCF32).Close(); err != nil {
			log.Fatalf("Could not close the IQ stream! %s", err.Error())
		}
	}
}

func (r *Radio) Destroy() {
	r.Stopping = true
	r.StreamDeactivate()
	r.StreamClose()
	close(*r.SamplesOutput)
}
