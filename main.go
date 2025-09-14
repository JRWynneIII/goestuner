package main

import (
	"github.com/alecthomas/kong"
	"github.com/charmbracelet/log"
	"github.com/jrwynneiii/goestuner/config"
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
			r := radio.New[complex64](rdef, rname, radio.CF32, xritDef.ChunkSize)
			r.Connect()
			d := demod.New(radio.CF32, float32(rdef.SampleRate), xritDef.ChunkSize, configFile)
			go d.Start()
			defer d.Close()
			go func() {
				var buf []complex64
				for {
					samples := r.Read(xritDef.ChunkSize)
					if len(samples.([]complex64)) >= int(xritDef.ChunkSize) {
						buf = samples.([]complex64)
					} else {
						buf = append(buf, samples.([]complex64)...)
					}

					if len(buf) >= int(xritDef.ChunkSize) {
						d.SampleInput <- samples.([]complex64)
						buf = []complex64{}
					}
				}
			}()

			for {
				select {
				case symbols := <-d.SymbolsOutput:
					log.Infof("Got symbols: %##v\n", symbols)
				}
			}

			r.Destroy()
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
