package radio

// #cgo CFLAGS: -g -Wall
// #cgo LDFLAGS: -lSoapySDR
import (
	"time"

	"github.com/charmbracelet/log"
	"github.com/jrwynneiii/goestuner/types"

	"github.com/pothosware/go-soapy-sdr/pkg/device"
)

type SDR struct {
	SamplesOutput *chan []complex64
	Driver        string
	DeviceIndex   string
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

func (r *SDR) Start() {
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

func New(conf types.Radio, bufSize uint, output *chan []complex64) *SDR {
	log.Debug("Initing SoapySDR")
	if _, err := InitSoapySDR(); err != nil {
		panic(err)
	}

	r := SDR{
		Driver:        conf.Driver,
		DeviceIndex:   conf.DeviceIndex,
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

func (r *SDR) Pause() {
	r.Stopping = true
	r.StreamDeactivate()
	r.StreamClose()
	r.BufferCF32 = make([][]complex64, 1)
	r.BufferCF32[0] = make([]complex64, r.chunksize)
}

func (r *SDR) Read(num uint) any {
	flags := make([]int, 1)
	timeout := uint(100000) //nanosec

	if !r.Stopping {
		timeNs, numSamples, err := r.stream.(*device.SDRStreamCF32).Read(r.BufferCF32, num, flags, timeout)
		log.Debugf("timeNs: %v, numSamples: %v, err: %v", timeNs, numSamples, err)
		return r.BufferCF32[0][:numSamples]
	}

	return []complex64{}
}

func (r *SDR) Connect() {
	r.args = make(map[string]string)
	r.args["driver"] = r.Driver
	if r.Driver == "rtltcp" {
		r.args["rtltcp"] = r.Address
	} else {
		r.args["index"] = r.DeviceIndex
	}
	// Create the soapysdr device object
	var err error
	if r.device == nil {
		if r.device, err = device.Make(r.args); err != nil {
			log.Fatalf("Could not create SoapySDR device (args=%#v)! %s", r.args, err.Error())
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

func (r *SDR) StreamActivate() {
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

func (r *SDR) StreamDeactivate() {
	log.Debug("Deactivating IQ stream...")
	if r.stream != nil {
		if err := r.stream.(*device.SDRStreamCF32).Deactivate(0, 0); err != nil {
			log.Fatalf("Could not deactivate the IQ stream! %s", err.Error())
		}
	}
}

func (r *SDR) StreamClose() {
	log.Debug("Closing IQ stream...")
	if r.stream != nil {
		if err := r.stream.(*device.SDRStreamCF32).Close(); err != nil {
			log.Fatalf("Could not close the IQ stream! %s", err.Error())
		}
	}
}

func (r *SDR) Destroy() {
	r.Stopping = true
	r.StreamDeactivate()
	r.StreamClose()
	close(*r.SamplesOutput)
}
