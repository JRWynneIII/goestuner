package decode

import (
	"time"

	"github.com/charmbracelet/log"
	"github.com/jrwynneiii/goestuner/config"
	"github.com/knadh/koanf/v2"
	SatHelper "github.com/opensatelliteproject/libsathelper"
)

var VCIDs = map[int]string{
	0:  "Admin Text",
	1:  "Mesoscale",
	2:  "GOES-ABI", // Band 2
	6:  "GOES15",
	7:  "GOES-ABI", // Band 7
	8:  "GOES-ABI", // Band 8
	9:  "GOES-ABI", // Band 8
	13: "GOES-ABI", // Band 13
	14: "GOES-ABI", // Band 14
	15: "GOES-ABI", // Band 15
	17: "GOES17",
	20: "EMWIN",
	21: "EMWIN",
	22: "EMWIN",
	23: "NWS",
	24: "NHC",
	25: "GOES16-JPG",
	26: "INTL",
	30: "DCS",
	31: "DCS",
	32: "DCS",
	60: "Himawari",
	63: "IDLE",
}

type Decoder struct {
	FrameLock                 bool
	SymbolsInput              chan byte
	MaxVitErrors              int
	ViterbiBytes              []byte
	DecodedBytes              []byte
	LastFrameSizeBits         int
	LastFrameSizeBytes        int
	LastFrameEnd              []byte
	Viterbi                   SatHelper.Viterbi27
	EncodedBytes              []byte
	ReedSolomonCorrectedBytes []byte
	ReedSolomanWorkBuffer     []byte
	ReedSolomon               SatHelper.ReedSolomon
	Correlator                SatHelper.Correlator
	PacketFixer               SatHelper.PacketFixer
	SyncWord                  []byte
	EncodedFrameSize          int
	DefaultFlywheelRecheck    int
	MinCorrelationBits        uint
	FrameSize                 int
	SyncWordSize              int
	RsBlocks                  byte
	RSWorkBuffer              []byte
	RSCorrectedData           []byte
	RSParityBlockSize         int
	RSParitySize              int
	AverageRsCorrections      float32
	AvgVitCorrections         float32
	SigQuality                float32
}

func New(bufsize uint, configFile *koanf.Koanf) *Decoder {
	var vitConf config.ViterbiConf
	var xritConf config.XRITFrameConf

	configFile.Unmarshal("viterbi", &vitConf)
	configFile.Unmarshal("xrit_frame", &xritConf)

	frameSizeBits := xritConf.FrameSize * 8
	encodedFrameSize := frameSizeBits * 2
	LastFrameSizeBits := xritConf.LastFrameSize * 8

	d := Decoder{
		FrameLock:              false,
		SymbolsInput:           make(chan byte, bufsize),
		ViterbiBytes:           make([]byte, encodedFrameSize+LastFrameSizeBits),
		DecodedBytes:           make([]byte, xritConf.FrameSize+xritConf.LastFrameSize), //?
		LastFrameEnd:           make([]byte, LastFrameSizeBits),
		EncodedBytes:           make([]byte, encodedFrameSize),
		SyncWord:               make([]byte, 4),
		RSWorkBuffer:           make([]byte, 255),
		RSCorrectedData:        make([]byte, xritConf.FrameSize),
		Viterbi:                SatHelper.NewViterbi27(frameSizeBits + LastFrameSizeBits),
		MaxVitErrors:           vitConf.MaxErrors,
		LastFrameSizeBits:      LastFrameSizeBits,
		LastFrameSizeBytes:     xritConf.LastFrameSize,
		ReedSolomon:            SatHelper.NewReedSolomon(),
		Correlator:             SatHelper.NewCorrelator(),
		PacketFixer:            SatHelper.NewPacketFixer(),
		EncodedFrameSize:       encodedFrameSize,
		DefaultFlywheelRecheck: 100,
		MinCorrelationBits:     46,
		FrameSize:              xritConf.FrameSize,
		SyncWordSize:           4,
		RsBlocks:               4,
		RSParityBlockSize:      32 * 4,
		RSParitySize:           32,
		AverageRsCorrections:   0.0,
		AvgVitCorrections:      0.0,
	}

	for i := 0; i < d.LastFrameSizeBits; i++ {
		d.LastFrameEnd[i] = 128
	}

	//Configure the ReedSolomon error corrector
	d.ReedSolomon.SetCopyParityToOutput(true)

	// Prime the correlator
	d.Correlator.AddWord(uint64(0xfc4ef4fd0cc2df89))
	d.Correlator.AddWord(uint64(0x25010b02f33d2076))

	return &d
}

func shiftWithConstantSize(arr *[]byte, pos int, length int) {
	for i := 0; i < length-pos; i++ {
		(*arr)[i] = (*arr)[pos+i]
	}
}

func (d *Decoder) Start() {
	var isCorrupted bool
	LastFrameOk := false

	//	var lostPacketsPerChannel [256]int64
	//	var lastPacketCount [256]int64
	//	var rxPacketsPerChannel [256]int64
	flywheelCount := 0
	for {
		//This is the meat and potatoes here. We should get our BER, SNR, and Sync status here
		if len(d.SymbolsInput) >= d.EncodedFrameSize {
			//Grab a frame's worth of symbols
			for i := 0; i < d.EncodedFrameSize; i++ {
				d.EncodedBytes[i] = <-d.SymbolsInput
			}

			if flywheelCount == d.DefaultFlywheelRecheck {
				LastFrameOk = false
				flywheelCount = 0
			}

			if !LastFrameOk {
				//Try to lock
				d.Correlator.Correlate(&d.EncodedBytes[0], uint(d.EncodedFrameSize))
			} else {
				//If we're already locked
				d.Correlator.Correlate(&d.EncodedBytes[0], uint(d.EncodedFrameSize/64))
				if d.Correlator.GetHighestCorrelationPosition() != 0 {
					//Lost lock
					d.Correlator.Correlate(&d.EncodedBytes[0], uint(d.EncodedFrameSize))
					flywheelCount = 0
				}
			}
			flywheelCount++

			//d.Correlator.GetCorrelationWordNumber()
			pos := d.Correlator.GetHighestCorrelationPosition()
			corr := d.Correlator.GetHighestCorrelation()

			if corr < d.MinCorrelationBits {
				log.Debugf("Correlation did not meet criteria: have: %d, want: %d", corr, d.MinCorrelationBits)
				LastFrameOk = false
				continue
			}

			if pos != 0 {
				shiftWithConstantSize(&d.EncodedBytes, int(pos), d.EncodedFrameSize)
				offset := uint(d.EncodedFrameSize) - pos
				for i := offset; i < uint(d.EncodedFrameSize); i++ {
					d.EncodedBytes[i] = <-d.SymbolsInput

				}
			}

			//Prepend the remaining bits from last chunk to vit data so we hopefully get a full packet?
			copy(d.ViterbiBytes[:d.LastFrameSizeBits], d.LastFrameEnd[:d.LastFrameSizeBits])
			//copy(d.ViterbiBytes[d.LastFrameSizeBits:d.EncodedFrameSize+d.LastFrameSizeBits], d.EncodedBytes[d.LastFrameSizeBits:d.EncodedFrameSize])
			for i := d.LastFrameSizeBits; i < d.LastFrameSizeBits+d.EncodedFrameSize; i++ {
				d.ViterbiBytes[i] = d.EncodedBytes[i-d.LastFrameSizeBits]
			}

			d.Viterbi.Decode(&d.ViterbiBytes[0], &d.DecodedBytes[0])

			nzrmDecodeSize := d.FrameSize + d.LastFrameSizeBytes
			SatHelper.DifferentialEncodingNrzmDecode(&d.DecodedBytes[0], nzrmDecodeSize)

			BER := d.Viterbi.GetBER()
			BER -= d.LastFrameSizeBits / 2

			if BER < 0 {
				BER = 0
			}

			signalQuality := 100 * ((float32(d.MaxVitErrors) - float32(BER)) / float32(d.MaxVitErrors))
			if signalQuality > 100 {
				signalQuality = 100
			} else if signalQuality < 0 {
				signalQuality = 0
			}

			d.SigQuality = signalQuality

			d.AvgVitCorrections += float32(BER)
			shiftWithConstantSize(&d.DecodedBytes, d.LastFrameSizeBytes/2, d.FrameSize+d.LastFrameSizeBytes/2)

			copy(d.LastFrameEnd[:d.LastFrameSizeBits], d.ViterbiBytes[d.EncodedFrameSize:d.EncodedFrameSize+d.LastFrameSizeBits])
			copy(d.SyncWord[:d.SyncWordSize], d.DecodedBytes[:d.SyncWordSize])

			shiftWithConstantSize(&d.DecodedBytes, d.SyncWordSize, d.FrameSize-d.SyncWordSize)

			SatHelper.DeRandomizerDeRandomize(&d.DecodedBytes[0], d.FrameSize-d.SyncWordSize)

			//Reed Solomon Time
			derrors := make([]int32, d.RsBlocks)
			totalBytesFixed := int32(0)

			for i := 0; i < int(d.RsBlocks); i++ {
				d.ReedSolomon.Deinterleave(&d.DecodedBytes[0], &d.RSWorkBuffer[0], byte(i), d.RsBlocks)
				derrors[i] = int32(int8(d.ReedSolomon.Decode_ccsds(&d.RSWorkBuffer[0])))

				d.ReedSolomon.Interleave(&d.RSWorkBuffer[0], &d.RSCorrectedData[0], byte(i), d.RsBlocks)

				if derrors[i] != -1 {
					d.AverageRsCorrections += float32(derrors[i])
				}
				if derrors[i] > -1 {
					totalBytesFixed += derrors[i]
				}
			}

			if derrors[0] == -1 && derrors[1] == -1 && derrors[2] == -1 && derrors[3] == -1 {
				isCorrupted = true
				LastFrameOk = false
			} else {
				isCorrupted = false
				LastFrameOk = true
			}

			//scid := ((d.RSCorrectedData[0] & 0x3F) << 2) | (d.RSCorrectedData[1]&0xC0)>>6
			//vcid := d.RSCorrectedData[1] & 0x3F

			counter := uint(d.RSCorrectedData[2])
			counter = SatHelper.ToolsSwapEndianess(counter)
			counter &= 0xFFFFFF00
			counter >>= 8

			if !isCorrupted {
				d.FrameLock = true
				//log.Infof("Got frame: vcid: %d (%s) scid: %d counter: %d", int(vcid), VCIDs[int(vcid)], scid, counter)
				//log.Infof("Packet data: %s", string(d.RSCorrectedData[:d.FrameSize-d.RSParityBlockSize-d.SyncWordSize]))
			} else {
				d.FrameLock = false
			}

		} else {
			time.Sleep(5 * time.Microsecond)
		}
	}
}
