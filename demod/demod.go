package demod

import (
	"math"
	"time"

	"github.com/charmbracelet/log"
	"github.com/jrwynneiii/goestuner/config"
	"github.com/jrwynneiii/goestuner/radio"
	"github.com/knadh/koanf/v2"
	SatHelper "github.com/opensatelliteproject/libsathelper"
	"github.com/racerxdl/segdsp/dsp"
	"github.com/racerxdl/segdsp/tools"
	"gonum.org/v1/gonum/dsp/fourier"
)

type Demodulator struct {
	SampleInput       chan []complex64
	SampleType        radio.StreamType
	SymbolsOutput     *chan byte
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
	CurrentFFT        []float64
	DoFFT             bool
}

func New(stype radio.StreamType, srate float32, bufsize uint, configFile *koanf.Koanf, decoderInput *chan byte) *Demodulator {
	xritConf := config.XRITConf{
		SymbolRate:             configFile.Float64("xrit.symbol_rate"),
		RRCAlpha:               configFile.Float64("xrit.rrc_alpha"),
		RRCTaps:                configFile.Int("xrit.rrc_taps"),
		LowPassTransitionWidth: configFile.Float64("xrit.lowpass_transition_width"),
		PLLAlpha:               float32(configFile.Float64("xrit.pll_alpha")),
		Decimation:             configFile.Int("xrit.decimation_factor"),
		ChunkSize:              uint(configFile.Int("xrit.chunk_size")),
		DoFFT:                  configFile.Bool("xrit.do_fft"),
	}
	agcConf := config.AGCConf{
		Rate:      float32(configFile.Float64("agc.rate")),
		Reference: float32(configFile.Float64("agc.reference")),
		Gain:      float32(configFile.Float64("agc.gain")),
		MaxGain:   float32(configFile.Float64("agc.max_gain")),
	}
	clockConf := config.ClockRecoveryConf{
		Mu:         float32(configFile.Float64("clockrecovery.mu")),
		Alpha:      float32(configFile.Float64("clockrecovery.alpha")),
		OmegaLimit: float32(configFile.Float64("clockrecovery.omega_limit")),
	}

	log.Debugf("Found xrit definition: %##v", xritConf)
	log.Debugf("Found agc definition: %##v", agcConf)
	log.Debugf("Found clock_recovery definition: %##v", clockConf)

	d := Demodulator{
		SampleInput:       make(chan []complex64, bufsize),
		SampleType:        stype,
		SymbolsOutput:     decoderInput,
		bufferSize:        bufsize,
		deviceSampleRate:  srate,
		circuitSampleRate: float32(srate) / float32(xritConf.Decimation),
		decimFactor:       xritConf.Decimation,
		sampleChunkSize:   int(xritConf.ChunkSize),
		gainOmega:         float32((clockConf.Alpha * clockConf.Alpha) / 4.0),
		DoFFT:             xritConf.DoFFT,
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

func (d *Demodulator) doFFT(samples []complex64) {
	var input []complex128

	for idx, sample := range samples {
		if idx%300 == 0 {
			input = append(input, complex128(sample))
		}
	}

	fft := fourier.NewCmplxFFT(len(input))
	coeff := fft.Coefficients(nil, input)

	var output []float64
	for i := range coeff {
		i = fft.ShiftIdx(i)
		v := tools.ComplexAbsSquared(complex64(coeff[i])) // * (1.0 / d.circuitSampleRate)
		v = float32(10.0 * math.Log10(float64(v)))
		output = append(output, float64(v))
	}

	d.CurrentFFT = output

	// fft_samples := fft.FFT(samples)
	// d.CurrentFFT = []float64{}
	//
	//	for _, sample := range fft_samples {
	//		value := 10 * math.Log10(float64(tools.ComplexAbsSquared(sample)))
	//		d.CurrentFFT = append(d.CurrentFFT, value)
	//	}
}

func (d *Demodulator) Start() {
	for {
		select {
		case samples := <-d.SampleInput:
			d.demodBlock(samples)
		default:
			time.Sleep(time.Millisecond)
		}
	}
}

func (d *Demodulator) demodBlock(samples []complex64) {
	length := len(samples)
	input := make([]complex64, length)

	if length <= 64*1024 {
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
	if d.DoFFT {
		d.doFFT(input)
	}

	//Apply AGC
	log.Debugf("[demod] Applying AGC")
	out := make([]complex64, length)
	d.AGC.Work(&input[0], &out[0], length)
	out = out[:length]

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

	for _, symbol := range symbols {
		*d.SymbolsOutput <- symbol
	}
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
}
