package radio

import (
	"fmt"

	"github.com/charmbracelet/log"
	"github.com/pothosware/go-soapy-sdr/pkg/device"
	"github.com/pothosware/go-soapy-sdr/pkg/modules"
	"github.com/pothosware/go-soapy-sdr/pkg/sdrlogger"
	"github.com/pothosware/go-soapy-sdr/pkg/version"
)

// Tracks overall information about the SoapySDR environment and available devices/modules
type SoapySubsystem struct {
	ABIVersion string
	APIVersion string
	LibVersion string
	Modules    []SoapyModule
	Devices    []SoapyDevice
}

type SoapyModule struct {
	Name    string
	Version string
}

type SoapyDevice struct {
	Driver string
	Device *device.SDRDevice
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

func (s *SoapySubsystem) GetDevicesByDriver(driver string) []*device.SDRDevice {
	var devs []*device.SDRDevice
	for _, dev := range s.Devices {
		if dev.Driver == driver {
			devs = append(devs, dev.Device)
		}
	}
	return devs
}

func InitSoapySDR() (*SoapySubsystem, error) {
	subsys := SoapySubsystem{
		ABIVersion: version.GetABIVersion(),
		APIVersion: version.GetAPIVersion(),
		LibVersion: version.GetLibVersion(),
	}

	// Tune down the SoapySDR logger so its not noisy
	sdrlogger.SetLogLevel(sdrlogger.Error)

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

	if err := subsys.EnumerateAllSDRs(); err != nil {
		panic(err)
	}

	return &subsys, nil
}

func (s *SoapySubsystem) EnumerateAllSDRs() error {
	devices := device.Enumerate(nil)
	args := make([]map[string]string, len(devices))
	//Construct our args list; basically every driver of every device we find
	for idx, dev := range devices {
		args[idx] = map[string]string{"driver": dev["driver"]}
	}

	//Make our device list
	if devs, err := device.MakeList(args); err == nil {
		for idx, dev := range devs {
			s.Devices = append(s.Devices, SoapyDevice{args[idx]["driver"], dev})
		}
	} else {
		return err
	}
	return nil
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

func (s *SoapySubsystem) PrintDeviceInfo(debug bool) {
	for _, dev := range s.Devices {
		if debug {
			log.Debugf("Driver: %s", dev.Driver)
		} else {
			log.Infof("Driver: %s", dev.Driver)
			LogAvailSettings(dev.Device)
		}
	}
}

func (s *SoapySubsystem) LogAllSoapySDRDevices() {
	// List Soapy library information
	s.PrintVersion(false)
	s.PrintSearchPaths(false)
	s.PrintModuleInfo(false)
	s.PrintDeviceInfo(false)
}
