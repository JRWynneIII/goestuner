package datalink

import (
	"fmt"
	"time"

	"github.com/charmbracelet/log"
	"github.com/jrwynneiii/goestuner/config"
	"github.com/knadh/koanf/v2"
	SatHelper "github.com/opensatelliteproject/libsathelper"
)

var VCIDs = map[int]string{
	0:  "Admin Text",
	1:  "Mesoscale",
	2:  "Visual",
	6:  "GOES-ABI",
	7:  "Shortwave IR",
	8:  "Mid-Level Water Vapor",
	9:  "Upper-Level Water Vapor",
	13: "Clean Long-Wave IR",
	14: "IR Long-Wave",
	15: "Dirty Long-Wave IR",
	17: "GOES18 - Clean Long-Wave IR",
	20: "EMWIN - High Priority",
	21: "EMWIN - Graphics",
	22: "EMWIN - Low Priority",
	23: "GOES-ABI",
	24: "NHC Maritime Graphics",
	25: "Other GOES-19 Graphics",
	26: "INTL",
	30: "DCS Admin",
	31: "DCS",
	32: "DCS (New Format)",
	60: "Himawari",
	63: "IDLE",
}

type Decoder struct {
	TotalFramesProcessed      int
	RxPacketsPerChannel       map[int]int
	DroppedPacketsPerChannel  map[int]int
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
	MaxRecheckThreshold       int
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

	lastFrameOk         bool
	recheckCounter      int
	currentFrameCorrupt bool
}

func (d *Decoder) Close() {
	close(d.SymbolsInput)
}

func New(bufsize uint, configFile *koanf.Koanf) *Decoder {
	vitConf := config.ViterbiConf{
		MaxErrors: configFile.Int("viterbi.max_errors"),
	}
	xritConf := config.XRITFrameConf{
		FrameSize:     configFile.Int("xritframe.frame_size"),
		LastFrameSize: configFile.Int("xritframe.last_frame_size"),
	}

	frameSizeBits := xritConf.FrameSize * 8
	encodedFrameSize := frameSizeBits * 2
	LastFrameSizeBits := xritConf.LastFrameSize * 8

	d := Decoder{
		TotalFramesProcessed:     0,
		RxPacketsPerChannel:      make(map[int]int),
		DroppedPacketsPerChannel: make(map[int]int),
		FrameLock:                false,
		SymbolsInput:             make(chan byte, bufsize),
		ViterbiBytes:             make([]byte, encodedFrameSize+LastFrameSizeBits),
		DecodedBytes:             make([]byte, xritConf.FrameSize+xritConf.LastFrameSize), //?
		LastFrameEnd:             make([]byte, LastFrameSizeBits),
		EncodedBytes:             make([]byte, encodedFrameSize),
		SyncWord:                 make([]byte, 4),
		RSWorkBuffer:             make([]byte, 255),
		RSCorrectedData:          make([]byte, xritConf.FrameSize),
		Viterbi:                  SatHelper.NewViterbi27(frameSizeBits + LastFrameSizeBits),
		MaxVitErrors:             vitConf.MaxErrors,
		LastFrameSizeBits:        LastFrameSizeBits,
		LastFrameSizeBytes:       xritConf.LastFrameSize,
		ReedSolomon:              SatHelper.NewReedSolomon(),
		Correlator:               SatHelper.NewCorrelator(),
		PacketFixer:              SatHelper.NewPacketFixer(),
		EncodedFrameSize:         encodedFrameSize,
		MaxRecheckThreshold:      100,
		MinCorrelationBits:       46,
		FrameSize:                xritConf.FrameSize,
		SyncWordSize:             4,
		RsBlocks:                 4,
		RSParityBlockSize:        32 * 4,
		RSParitySize:             32,
		AverageRsCorrections:     0.0,
		AvgVitCorrections:        0.0,
		lastFrameOk:              false,
		recheckCounter:           0,
		currentFrameCorrupt:      false,
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

func (d *Decoder) checkIfFrameLocked() {
	// Use the correlator to see where the sync words are in the frame, such that we know where the packet starts
	// If we're not frame locked, or we've gotten a lot of good packets and should make sure were on the right
	// track and not out of sync, then try to recorrelate, otherwise, don't try and recorrelate the whole frame
	if !d.lastFrameOk || d.recheckCounter >= d.MaxRecheckThreshold {
		d.Correlator.Correlate(&d.EncodedBytes[0], uint(d.EncodedFrameSize))
		d.recheckCounter = 0
		d.lastFrameOk = false
	} else {
		//If we're already locked
		d.Correlator.Correlate(&d.EncodedBytes[0], uint(d.EncodedFrameSize/64))
		if d.Correlator.GetHighestCorrelationPosition() != 0 {
			//Lost lock, so lets recorrelate the whole frame
			d.Correlator.Correlate(&d.EncodedBytes[0], uint(d.EncodedFrameSize))
			d.recheckCounter = 0
		}
	}
	d.recheckCounter++
}

func (d *Decoder) correlate() error {
	// Check to make sure we actually got enough data that contains a packet/frame
	if correlation := d.Correlator.GetHighestCorrelation(); correlation < d.MinCorrelationBits {
		log.Debugf("Correlation did not meet criteria: have: %d, want: %d", correlation, d.MinCorrelationBits)
		d.lastFrameOk = false
		return fmt.Errorf("No packet lock")
	}

	// Get the beginning of th epacket and shift things to start there
	if pos := d.Correlator.GetHighestCorrelationPosition(); pos != 0 {
		// Shift buffer to realign the frame to where the correlator says the frame begins
		copy(d.EncodedBytes[:d.EncodedFrameSize-int(pos)], d.EncodedBytes[int(pos):d.EncodedFrameSize])

		// Backfill bytes from the input channel to make a full frame
		offset := uint(d.EncodedFrameSize) - pos
		for i := offset; i < uint(d.EncodedFrameSize); i++ {
			d.EncodedBytes[i] = <-d.SymbolsInput

		}
	}
	return nil
}

func (d *Decoder) convolutionalDecode() {
	// Prepend the remaining bits from last chunk to vit data so that the viterbi problem space is larger
	// And therefore more likely to get a lock/decode
	copy(d.ViterbiBytes[:d.LastFrameSizeBits], d.LastFrameEnd[:d.LastFrameSizeBits])
	copy(d.ViterbiBytes[d.LastFrameSizeBits:], d.EncodedBytes[:d.EncodedFrameSize])

	d.Viterbi.Decode(&d.ViterbiBytes[0], &d.DecodedBytes[0])

}

func (d *Decoder) calculateBitErrorRate() int {
	// Track bit error rate
	BER := d.Viterbi.GetBER() - (d.LastFrameSizeBits / 2)
	if BER < 0 {
		BER = 0
	}
	d.AvgVitCorrections += float32(BER)
	return BER
}

func (d *Decoder) cleanFrame() {
	// Shift the decoded bytes to cut out half of the 'last frame' bytes
	// NOTE: not exactly sure why we do that, but it has something to do with incorporating the previous frame's
	// 	 bytes into things
	copy(d.DecodedBytes[:(d.FrameSize+d.LastFrameSizeBytes/2)-(d.LastFrameSizeBytes/2)], d.DecodedBytes[(d.LastFrameSizeBytes/2):(d.FrameSize+d.LastFrameSizeBytes/2)])

	// Keep the last viterbi encoded bytes from this packet so that we can expand our problem space for the next
	// packet's viterbi decode
	// My understanding is this allows us to have a better chance at the viterbi decode working
	copy(d.LastFrameEnd[:d.LastFrameSizeBits], d.ViterbiBytes[d.EncodedFrameSize:d.EncodedFrameSize+d.LastFrameSizeBits])
	// Get the next sync words from the decoded frame.
	copy(d.SyncWord[:d.SyncWordSize], d.DecodedBytes[:d.SyncWordSize])

	// Shift the decoded bytes to remove the sync words, so we should just have a clean packet frame now
	copy(d.DecodedBytes[:(d.FrameSize-d.SyncWordSize)-d.SyncWordSize], d.DecodedBytes[d.SyncWordSize:(d.FrameSize-d.SyncWordSize)])
}

func (d *Decoder) errorCorrectPacket() int32 {
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
		// Packet is corrupt; :sadpanda:
		d.currentFrameCorrupt = true
		d.lastFrameOk = false
	} else {
		// Got a good packet! lets go!
		d.currentFrameCorrupt = false
		d.lastFrameOk = true
	}

	return totalBytesFixed

}

func (d *Decoder) Start() {
	for {
		//This is the meat and potatoes here. We should get our BER, SNR, and Sync status here
		if len(d.SymbolsInput) >= d.EncodedFrameSize {
			//Grab a frame's worth of symbols
			for i := 0; i < d.EncodedFrameSize; i++ {
				d.EncodedBytes[i] = <-d.SymbolsInput
			}

			//Do we have frame sync?
			d.checkIfFrameLocked()

			//Find beginning of frame
			if err := d.correlate(); err != nil {
				// If the correlation errored, we don't have a good frame, so skip to next iteration
				continue
			}

			//Decode convolutional encoding
			d.convolutionalDecode()

			//Now lets do the differential decode
			SatHelper.DifferentialEncodingNrzmDecode(&d.DecodedBytes[0], d.FrameSize+d.LastFrameSizeBytes)

			BER := d.calculateBitErrorRate()

			// Calculate our 'signal quality' percentage based upon the bit error rate
			d.SigQuality = 100 * ((float32(d.MaxVitErrors) - float32(BER)) / float32(d.MaxVitErrors))
			if d.SigQuality > 100 {
				d.SigQuality = 100
			} else if d.SigQuality < 0 {
				d.SigQuality = 0
			}

			d.cleanFrame()

			// Derandomize packet: Unsure what exactly this does tbh
			SatHelper.DeRandomizerDeRandomize(&d.DecodedBytes[0], d.FrameSize-d.SyncWordSize)

			totalBytesFixed := d.errorCorrectPacket()

			d.TotalFramesProcessed++

			// Spacecraft ID (TODO: This seems to always be 0 for some reason?)
			scid := ((d.RSCorrectedData[0] & 0x3F) << 2) | (d.RSCorrectedData[1]&0xC0)>>6

			// Virtual Channel ID
			vcid := d.RSCorrectedData[1] & 0x3F

			counter := uint(d.RSCorrectedData[2])
			counter = SatHelper.ToolsSwapEndianess(counter)
			counter &= 0xFFFFFF00
			counter >>= 8

			if !d.currentFrameCorrupt {
				d.FrameLock = true
				log.Infof("Got frame: vcid: %d (%s) scid: %d object number: %d", int(vcid), VCIDs[int(vcid)], scid, counter)
				if totalBytesFixed > 0 {
					log.Infof("Parity corrected %d bytes from last packet", totalBytesFixed)
				}
				d.RxPacketsPerChannel[int(vcid)]++
			} else {
				d.DroppedPacketsPerChannel[int(vcid)]++
				d.FrameLock = false
			}

		} else {
			// Not enough symbols available, so lets sleep on it
			time.Sleep(5 * time.Microsecond)
		}
	}
}
