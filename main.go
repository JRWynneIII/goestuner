package main

import (
	"io/ioutil"
	"strconv"
	"strings"
	"time"

	"github.com/alecthomas/kong"
	"github.com/charmbracelet/log"
	"github.com/jrwynneiii/goestuner/config"
	"github.com/jrwynneiii/goestuner/decode"
	"github.com/jrwynneiii/goestuner/demod"
	"github.com/jrwynneiii/goestuner/radio"

	//"github.com/go-viper/encoding/hcl"

	"github.com/knadh/koanf/parsers/hcl"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/v2"
)

var configFile = koanf.New(".")

func main() {
	flags := kong.Parse(&cli)
	if cli.Verbose {
		log.SetLevel(log.DebugLevel)
	}

	//InitConfig()
	if err := configFile.Load(file.Provider("./config.hcl"), hcl.Parser(true)); err != nil {
		log.Fatalf("Could not read config file: %s", err.Error())
	}

	switch flags.Command() {
	case "tune":
		log.Info("Starting GOESWatcher")
		rname := configFile.String("radio.driver")

		var rdef config.RadioConf
		var xritDef config.XRITConf
		configFile.Unmarshal("radio", &rdef)
		configFile.Unmarshal("xrit", &xritDef)

		log.Infof("Found radio definition for %s: %##v", rname, rdef)
		log.Info("Starting init of SDR")
		switch rdef.SampleType {
		case "uint8":
			r := radio.New[uint8](rdef, rname, radio.CU8, xritDef.ChunkSize)
			r.Connect()
			r.Read(100)
			r.Destroy()
		case "int8":
			r := radio.New[int8](rdef, rname, radio.CS8, xritDef.ChunkSize)
			r.Connect()
			log.Debugf("%##v", r.Read(100))
			r.Destroy()
		case "uint16":
			r := radio.New[uint16](rdef, rname, radio.CU16, xritDef.ChunkSize)
			r.Connect()
			r.Read(100)
			r.Destroy()
		case "int16":
			r := radio.New[int16](rdef, rname, radio.CS16, xritDef.ChunkSize)
			r.Connect()
			r.Read(100)
			r.Destroy()
		case "complex64":
			if len(cli.Tune.File) == 0 {
				r := radio.New[complex64](rdef, rname, radio.CF32, xritDef.ChunkSize)
				r.Connect()
				defer r.Destroy()
				demodulator := demod.New(radio.CF32, float32(rdef.SampleRate), xritDef.ChunkSize, configFile)
				decoder := decode.New(xritDef.ChunkSize, configFile)
				go decoder.Start()
				go demodulator.Start()
				defer demodulator.Close()
				//Thread to get samples from the radio, and pass it to the demodulator
				go func() {
					var buf []complex64
					for {
						samples := r.Read(xritDef.ChunkSize)
						//demod.DumpSamples(samples.([]complex64))
						buf = append(buf, samples.([]complex64)...)

						if len(buf) >= int(xritDef.ChunkSize) {
							//log.Infof("Passing %d (chunksize=%d) samples to demodulator", len(buf), int(xritDef.ChunkSize))
							demodulator.SampleInput <- buf
							buf = []complex64{}
						}
						time.Sleep(5 * time.Millisecond)
					}
				}()

				// Thread to check output from demodulator to pass to decoder
				// TODO: probably build this feature into the demodulator; just pass the channel for the deocder
				// input into the demodulator and have it populate the channel instead of it's output chan
				go func() {
					for {
						select {
						case symbols := <-demodulator.SymbolsOutput:
							//log.Infof("Got symbols: %##v", symbols)
							for _, symbol := range symbols {
								decoder.SymbolsInput <- symbol
							}
						default:
							time.Sleep(time.Millisecond)
						}
					}
				}()
				for {
					log.Infof("Frame Lock: %v, Signal Quality: %f, BER: %f %%, avgVitCorrections: %f, avgRsCorrections: %f",
						decoder.FrameLock,
						decoder.SigQuality,
						decoder.Viterbi.GetPercentBER(),
						float64(decoder.AvgVitCorrections),
						float64(decoder.AverageRsCorrections))
					log.Infof("Queue usage: demod input: %d, demod output: %d, decoder input: %d",
						len(demodulator.SampleInput),
						len(demodulator.SymbolsOutput),
						len(decoder.SymbolsInput))
					time.Sleep(time.Second)
				}
			} else {
				if data, err := ioutil.ReadFile(cli.Tune.File); err == nil {
					log.Info("Parsing file")
					demodulator := demod.New(radio.CF32, float32(rdef.SampleRate), xritDef.ChunkSize, configFile)
					go demodulator.Start()
					decoder := decode.New(xritDef.ChunkSize, configFile)
					go decoder.Start()
					defer demodulator.Close()

					go func() {
						lines := strings.Split(string(data), "\n")
						var samples []complex64
						for _, line := range lines {
							vals := strings.Split(line, " ")
							re_s := vals[0]
							im_s := vals[0]
							var re, im float32
							if val, err := strconv.ParseFloat(re_s, 32); err != nil {
								log.Errorf("Could not parse %s", re_s)
							} else {
								re = float32(val)
							}
							if val, err := strconv.ParseFloat(im_s, 32); err != nil {
								log.Errorf("Could not parse %s", im_s)
							} else {
								im = float32(val)
							}
							samples = append(samples, complex64(complex(re, im)))
							if len(samples) >= int(xritDef.ChunkSize) {
								demodulator.SampleInput <- samples
								samples = []complex64{}
							}

						}
					}()
					// Thread to check output from demodulator to pass to decoder
					// TODO: probably build this feature into the demodulator; just pass the channel for the deocder
					// input into the demodulator and have it populate the channel instead of it's output chan
					go func() {
						for {
							select {
							case symbols := <-demodulator.SymbolsOutput:
								//log.Infof("Got symbols: %##v", symbols)
								for _, symbol := range symbols {
									decoder.SymbolsInput <- symbol
								}
							default:
								time.Sleep(time.Millisecond)
							}
						}
					}()
					for {
						log.Infof("Frame Lock: %v, Signal Quality: %f, BER: %f %%, avgVitCorrections: %f, avgRsCorrections: %f",
							decoder.FrameLock,
							decoder.SigQuality,
							decoder.Viterbi.GetPercentBER(),
							float64(decoder.AvgVitCorrections),
							float64(decoder.AverageRsCorrections))
						time.Sleep(time.Second)
					}
				} else {
					log.Errorf("Could not read file %s", cli.Tune.File)
				}
			}
		case "complex128":
			r := radio.New[complex128](rdef, rname, radio.CF64, xritDef.ChunkSize)
			r.Connect()
			r.Read(100)
			r.Destroy()
		default:
			log.Fatalf("Invalid sample_type defined for radio %s", rname)
		}
	default:
		log.Info("Command not recognized")
	}
}
