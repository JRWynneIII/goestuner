package demod

import (
	"fmt"
	"time"

	"github.com/charmbracelet/log"
	"github.com/jrwynneiii/goestuner/config"
	"github.com/jrwynneiii/goestuner/radio"
	"github.com/knadh/koanf/v2"
	SatHelper "github.com/opensatelliteproject/libsathelper"
	"github.com/racerxdl/segdsp/dsp"
)

type Demodulator struct {
	SampleInput       chan []complex64
	SampleType        radio.StreamType
	SymbolsOutput     chan []uint8
	bufferSize        uint
	circuitSampleRate float32
	deviceSampleRate  float32
	sps               float32
	decimFactor       int
	sampleChunkSize   int
	gainOmega         float32
	AGC               SatHelper.AGC
	ClockRecovery     SatHelper.ClockRecovery
	RRCFilter         *dsp.FirFilter
	Decimator         *dsp.FirFilter
	CostasLoop        dsp.CostasLoop
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

	d.AGC = SatHelper.NewAGC(agcConf.Rate, agcConf.Reference, agcConf.Gain, agcConf.MaxGain)
	d.ClockRecovery = SatHelper.NewClockRecovery(d.sps, (clockConf.Alpha*clockConf.Alpha)/4.0, clockConf.Mu, clockConf.Alpha, clockConf.OmegaLimit)
	d.RRCFilter = dsp.MakeFirFilter(dsp.MakeRRC(1, float64(srate), xritConf.SymbolRate, xritConf.RRCAlpha, xritConf.RRCTaps))
	d.Decimator = dsp.MakeDecimationFirFilter(int(xritConf.Decimation), dsp.MakeLowPass(1, float64(srate), float64(d.circuitSampleRate/2)-xritConf.LowPassTransitionWidth/2, xritConf.LowPassTransitionWidth))
	d.CostasLoop = dsp.MakeCostasLoop2(xritConf.PLLAlpha)

	return &d
}

func (d *Demodulator) Start() {
	for {
		select {
		case samples := <-d.SampleInput:
			//log.Infof("[demod]: Got %d samples ", len(samples))
			d.demodBlock(samples)
		default:
			time.Sleep(time.Millisecond)
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

func DumpSamples(samples []complex64) {
	for _, val := range samples {
		fmt.Printf("%f %f\n", real(val), imag(val))
	}
}

func (d *Demodulator) demodBlock(samples []complex64) {
	length := len(samples)
	input := make([]complex64, length)

	if length <= 64*1024 {
		log.Errorf("[demod] samples length is %d, not >= %d. Bailing!", length, 64*1024)
		return
	}

	//Populate our workbuffer0
	for idx, sample := range samples {
		input[idx] = sample
	}

	if d.decimFactor > 1 {
		log.Debugf("[demod] Running Decimator")
		input = d.Decimator.Work(input)
	}

	//Apply AGC
	log.Debugf("[demod] Applying AGC")
	out := make([]complex64, length)
	d.AGC.Work(&input[0], &out[0], length)
	out = out[:length]
	//out := d.AGC.Work(input)

	//Apply Filter
	log.Debugf("[demod] Applying RRC Filter")
	out = d.RRCFilter.Work(out)

	//Frequency Sync
	log.Debugf("[demod] Running Costas Loop")
	out = d.CostasLoop.Work(out)

	//Clock Sync
	log.Debugf("[demod] Running Clock Sync (length: %d, mu: %f, omega: %f)", length, d.ClockRecovery.GetMu(), d.ClockRecovery.GetOmega())
	syncd := make([]complex64, len(out))
	numSymbols := d.ClockRecovery.Work(&out[0], &syncd[0], len(out))

	symbols := d.processSymbols(syncd, numSymbols)

	d.SymbolsOutput <- symbols
}

func (d *Demodulator) processSymbols(ob []complex64, numSymbols int) []byte {
	log.Debugf("[demod] Processing symbols")

	symbols := make([]byte, numSymbols)
	for i := 0; i < numSymbols; i++ {
		val := ob[i]
		sym := real(val) * 127
		if sym > 127 {
			sym = 127
		} else if sym < -128 {
			sym = -128
		}
		symbols[i] = byte(sym)
	}
	return symbols
}

func (d *Demodulator) Close() {
	close(d.SampleInput)
	close(d.SymbolsOutput)
}
