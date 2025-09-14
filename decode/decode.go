package decode

import (
	"github.com/charmbracelet/log"
	"github.com/jrwynneiii/goestuner/config"
	"github.com/knadh/koanf/v2"
	SatHelper "github.com/opensatelliteproject/libsathelper"
)

type Decoder struct {
	SymbolsInput              chan []complex64
	SymbolsOutput             chan []complex64
	MaxVitErrors              int
	viterbiBytes              []byte
	decodedBytes              []byte
	lastFrameBytes            []byte
	Viterbi                   SatHelper.Viterbi
	encodedBytes              []byte
	ReedSolomanCorrectedBytes []byte
	reedSolomanWorkBuffer     []byte
	ReedSoloman               SatHelper.ReedSoloman
	Correlator                SatHelper.Correlator
	PacketFixer               SatHelper.PacketFixer
	syncWord                  []byte
}

func New(bufsize uint, configFile *koanf.Koanf) *Decoder {
	var vitConf config.ViterbiConf
	var xritConf config.XRITFrameConf

	configFile.Unmarshal("viterbi", &vitConf)
	configFile.Unmarshal("xrit_frame", &xritConf)

	frameSizeBits := xritConf.FrameSize * 8
	encodedFrameSize := frameSizeBits * 2
	lastFrameSizeBits := xritConf.LastFrameSize * 8

	d := Decoder{
		SymbolsInput:              make(chan []complex64, bufsize),
		SymbolsOutput:             make(chan []complex64, bufsize),
		MaxVitErrors:              vitConf.MaxErrors,
		viterbiBytes:              make([]byte, encodedFrameSize+lastFrameSizeBits),
		decodedBytes:              make([]byte, xritConf.FrameSize+xritConf.LastFrameSize), //?
		lastFrameBytes:            make([]byte, lastFrameSizeBits),
		Viterbi:                   SatHelper.NewViterbi27(frameSizeBits + lastFrameSizeBits),
		encodedBytes:              make([]byte, encodedFrameSize),
		ReedSolomanCorrectedBytes: make([]byte, xritConf.FrameSize),
		reedSolomanWorkBuffer:     make([]byte, 255),
		ReedSoloman:               SatHelper.NewReedSoloman(),
		Correlator:                SatHelper.NewCorrelator(),
		PacketFixer:               SatHelper.NewPacketFixer(),
		syncWord:                  make([]byte, 4),
	}

	//Configure the ReedSoloman error corrector
	d.ReedSoloman.SetCopyParityToOutput(true)

	// Prime the correlator
	d.Correlator.AddWord(0xfc4ef4fd0cc2df89)
	d.Correlator.AddWord(0x25010b02f33d2076)

	return &d
}

func (d *Decoder) Start() {
	for {
		select {
		case symbols := <-d.SymbolsInput:
			log.Debugf("[decode]: Got %d symbols to decode", len(symbols))
			d.decodeBlock(symbols)
		}
	}
}

func (d *Decoder) decodeBlock(symbols []complex64) {
	//This is the meat and potatoes here. We should get our BER, SNR, and Sync status here
}
