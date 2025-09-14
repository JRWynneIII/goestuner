package demod

import (
	"fmt"

	"github.com/charmbracelet/log"
	"github.com/jrwynneiii/goestuner/config"
	"github.com/jrwynneiii/goestuner/radio"
	"github.com/knadh/koanf/v2"
	SatHelper "github.com/opensatelliteproject/libsathelper"
	"github.com/racerxdl/segdsp/dsp"
)

type Demodulator struct {
	SampleInput       chan []complex64
	WorkBuffer0       []complex64
	WorkBuffer1       []complex64
	SampleType        radio.StreamType
	SymbolsOutput     chan []uint8
	bufferSize        uint
	circuitSampleRate float32
	deviceSampleRate  float32
	sps               float32
	decimFactor       int
	sampleChunkSize   int
	gainOmega         float32
	AGC               *dsp.SimpleAGC
	SHAGC             SatHelper.AGC
	//ClockRecovery     *digital.ComplexClockRecovery
	ClockRecovery SatHelper.ClockRecovery
	RRCFilter     *dsp.FirFilter
	Decimator     *dsp.FirFilter
	CostasLoop    dsp.CostasLoop
}

func New(stype radio.StreamType, srate float32, bufsize uint, configFile *koanf.Koanf) *Demodulator {
	var xritConf config.XRITConf
	var agcConf config.AGCConf
	var clockConf config.ClockRecoveryConf
	configFile.Unmarshal("xrit", &xritConf)
	configFile.Unmarshal("agc", &agcConf)
	configFile.Unmarshal("clock_recovery", &clockConf)
	log.Infof("Found xrit definition: %##v", xritConf)
	log.Infof("Found agc definition: %##v", agcConf)
	log.Infof("Found clock_recovery definition: %##v", clockConf)

	d := Demodulator{
		SampleInput:       make(chan []complex64, bufsize),
		WorkBuffer0:       make([]complex64, xritConf.ChunkSize),
		WorkBuffer1:       make([]complex64, xritConf.ChunkSize),
		SampleType:        stype,
		SymbolsOutput:     make(chan []byte, bufsize),
		bufferSize:        bufsize,
		deviceSampleRate:  srate,
		circuitSampleRate: float32(srate) / float32(xritConf.Decimation),
		decimFactor:       xritConf.Decimation,
		sampleChunkSize:   int(xritConf.ChunkSize),
		gainOmega:         float32((clockConf.Alpha * clockConf.Alpha) / 4.0),
	}
	d.sps = d.circuitSampleRate / float32(xritConf.SymbolRate)

	log.Debugf("Setting demodulator values: %##v", d)

	d.SHAGC = SatHelper.NewAGC(agcConf.Rate, agcConf.Reference, agcConf.Gain, agcConf.MaxGain)
	d.AGC = dsp.MakeSimpleAGC(agcConf.Rate, agcConf.Reference, agcConf.Gain, agcConf.MaxGain)
	d.ClockRecovery = SatHelper.NewClockRecovery(d.sps, (clockConf.Alpha*clockConf.Alpha)/4.0, clockConf.Mu, clockConf.Alpha, clockConf.OmegaLimit)
	//d.ClockRecovery = digital.NewComplexClockRecovery(d.sps,
	//	d.gainOmega,
	//	clockConf.Mu,
	//	clockConf.Alpha,
	//	clockConf.OmegaLimit)
	d.RRCFilter = dsp.MakeFirFilter(dsp.MakeRRC(1, float64(srate), xritConf.SymbolRate, xritConf.RRCAlpha, xritConf.RRCTaps))
	d.Decimator = dsp.MakeDecimationFirFilter(int(xritConf.Decimation), dsp.MakeLowPass(1, float64(srate), float64(d.circuitSampleRate/2)-xritConf.LowPassTransitionWidth/2, xritConf.LowPassTransitionWidth))
	d.CostasLoop = dsp.MakeCostasLoop2(xritConf.PLLAlpha)

	return &d
}

func (d *Demodulator) Start() {
	for {
		select {
		case samples := <-d.SampleInput:
			log.Debugf("[demod]: Got %d samples ", len(samples))
			d.demodBlock(samples)
		default:
			log.Debugf("Underflow")
		}
	}
}

func swapAndTrim(a *[]complex64, b *[]complex64, length int) {
	*a = (*a)[:length]
	*b = (*b)[:length]

	c := *b
	*b = *a
	*a = c
}

func dumpSamples(samples []complex64) {
	for _, val := range samples {
		fmt.Printf("%f %f\n", real(val), imag(val))
	}
}

func (d *Demodulator) checkAndResizeBuffers(length int) {
	if len(d.WorkBuffer0) < length {
		d.WorkBuffer0 = make([]complex64, length)
	}

	if len(d.WorkBuffer1) < length {
		d.WorkBuffer1 = make([]complex64, length)
	}
}

func (d *Demodulator) demodBlock(samples []complex64) {
	length := len(samples)

	if length <= 64*1024 {
		log.Errorf("[demod] samples length is %d, not >= %d. Bailing!", length, 64*1024)
		return
	}

	d.checkAndResizeBuffers(length)

	//Populate our workbuffer0
	for idx, sample := range samples {
		d.WorkBuffer0[idx] = sample
	}

	ban := d.WorkBuffer0
	bbn := d.WorkBuffer1
	ba := &ban[0]
	bb := &bbn[0]

	if d.decimFactor > 1 {
		log.Debugf("[demod] Running Decimator")
		length = d.Decimator.WorkBuffer(ban, bbn)
		swapAndTrim(&ban, &bbn, length)
		ba = &ban[0]
		bb = &bbn[0]
	}

	//Apply AGC
	log.Debugf("[demod] Applying AGC")
	d.SHAGC.Work(ba, bb, length)
	swapAndTrim(&ban, &bbn, length)

	//Apply Filter
	log.Debugf("[demod] Applying RRC Filter")
	length = d.RRCFilter.WorkBuffer(ban, bbn)
	swapAndTrim(&ban, &bbn, length)

	//Frequency Sync
	log.Debugf("[demod] Running Costas Loop")
	length = d.CostasLoop.WorkBuffer(ban, bbn)
	swapAndTrim(&ban, &bbn, length)
	ba = &ban[0]
	bb = &bbn[0]

	//Clock Sync
	log.Debugf("[demod] Running Clock Sync (length: %d, mu: %f, omega: %f)", length, d.ClockRecovery.GetMu(), d.ClockRecovery.GetOmega())
	numSymbols := d.ClockRecovery.Work(ba, bb, length)
	//numSymbols := d.ClockRecovery.WorkBuffer(ban, bbn)
	swapAndTrim(&ban, &bbn, length)
	ba = &ban[0]

	log.Debugf("[demod] Processing symbols")
	var ob *[]complex64

	if ba == &d.WorkBuffer0[0] {
		ob = &d.WorkBuffer0
	} else {
		ob = &d.WorkBuffer1
	}

	symbols := make([]byte, numSymbols)
	for i := 0; i < numSymbols; i++ {
		z := (*ob)[i]
		v := real(z) * 127
		if v > 127 {
			v = 127
		} else if v < -128 {
			v = -128
		}
		symbols[i] = byte(v)
	}
	d.SymbolsOutput <- symbols
}

func (d *Demodulator) Close() {
	close(d.SampleInput)
	close(d.SymbolsOutput)
}
