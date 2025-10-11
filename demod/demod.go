package demod

import (
	"math"
	"math/cmplx"
	"sync"
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

type SNRCalc struct {
	Y1     float64
	Y2     float64
	Alpha  float64
	Beta   float64
	Signal float64
	Noise  float64
}

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
	FFTWorking        bool
	Stopping          bool
	FFTMutex          sync.RWMutex
	SNR               *SNRCalc
	CurrentSNR        float64
	PeakSNR           float64
	AvgSNR            float64
}

func NewSNRCalc() *SNRCalc {
	alpha := 0.001
	s := SNRCalc{
		Y1:     0,
		Y2:     0,
		Signal: 0,
		Noise:  0,
		Alpha:  alpha,
		Beta:   1.0 - alpha,
	}
	return &s
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
		SNR:               NewSNRCalc(),
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

func trimSlice(s []complex64) []complex64 {
	if len(s) > 0 {
		lastZero := -1
		for i := len(s) - 1; i >= 0; i-- {
			if s[i] != 0+0i {
				break
			}
			lastZero = i
		}
		if lastZero == -1 {
			return s
		}
		return s[:lastZero+1]
	}
	return s
}

// The SNR calculation routine is based upon SatDump's SNR calculation routine found at:
// https://github.com/SatDump/SatDump/blob/master/src-core/common/dsp/utils/snr_estimator.cpp
// Which in turn is based upon the following paper:
//
// D. R. Pauluzzi and N. C. Beaulieu, "A comparison of SNR
// estimation techniques for the AWGN channel," IEEE
// Trans. Communications, Vol. 48, No. 10, pp. 1681-1691, 2000.
//
// TODO Break this out into a separate file, and move to a method on the SNR object
func (d *Demodulator) GetSNR(s *[]complex64) float64 {
	for _, samp := range *s {
		tmp_y1 := math.Pow(cmplx.Abs(complex128(samp)), 2)
		d.SNR.Y1 = d.SNR.Alpha*tmp_y1 + d.SNR.Beta*d.SNR.Y1

		tmp_y2 := math.Pow(cmplx.Abs(complex128(samp)), 4)
		d.SNR.Y2 = d.SNR.Alpha*tmp_y2 + d.SNR.Beta*d.SNR.Y2
	}

	if math.IsNaN(d.SNR.Y1) {
		d.SNR.Y1 = 0.0
	}

	if math.IsNaN(d.SNR.Y2) {
		d.SNR.Y2 = 0.0
	}

	y1_2 := math.Pow(d.SNR.Y1, 2)
	// Breaking out radicand here to avoid any floating point errors, since
	// we sqrt it twice
	radicand := 2.0*y1_2 - d.SNR.Y2
	d.SNR.Signal = math.Sqrt(radicand)
	d.SNR.Noise = d.SNR.Y1 - math.Sqrt(radicand)

	return max(0, 10.0*math.Log10(d.SNR.Signal/d.SNR.Noise))
}

func (d *Demodulator) doFFT(samples []complex64) {
	d.FFTWorking = true
	var input []complex128

	for _, sample := range samples {
		input = append(input, complex128(sample))
	}

	fft := fourier.NewCmplxFFT(len(input))
	coeff := fft.Coefficients(nil, input)

	var output []float64
	for i := range coeff {
		// Cut this down to a manageable size
		if i%1000 == 0 {
			i = fft.ShiftIdx(i)
			v := tools.ComplexAbsSquared(complex64(coeff[i])) // * (1.0 / d.circuitSampleRate)
			v = float32(10.0 * math.Log10(float64(v)))
			if v > 0 {
				output = append(output, float64(v))
			}
		}
	}

	d.FFTMutex.Lock()
	d.CurrentFFT = output
	d.FFTMutex.Unlock()

	time.Sleep(500 * time.Millisecond)
	d.FFTMutex.Lock()
	d.FFTWorking = false
	d.FFTMutex.Unlock()
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

	//Trim out any extra space we have
	//NOTE: This may or may not be a good idea, but it allows our SNR calculator to actually work
	//	If this causes an issue with the datalink layer, then lets move the trim to the SNR object
	syncd = trimSlice(syncd)

	// Update our SNR values in the demodulator
	snr := d.GetSNR(&syncd)

	if snr > d.PeakSNR {
		d.PeakSNR = snr
	}

	//To avoid strange NaN's for average
	if snr > 0 {
		d.AvgSNR += snr
		d.AvgSNR /= 2
	}

	d.CurrentSNR = snr

	// Do the FFT things
	d.FFTMutex.RLock()
	if d.DoFFT && !d.FFTWorking {
		d.FFTMutex.RUnlock()
		go d.doFFT(out)
	} else {
		d.FFTMutex.RUnlock()
	}

	symbols := d.processSymbols(syncd, numSymbols)

	for _, symbol := range symbols {
		if !d.Stopping {
			*d.SymbolsOutput <- symbol
		}
	}
}

func (d *Demodulator) processSymbols(ob []complex64, numSymbols int) []byte {
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
	d.Stopping = true
	close(*d.SymbolsOutput)
}
