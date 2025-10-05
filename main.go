package main

import (
	"errors"
	"fmt"
	"os"
	"runtime/pprof"
	"strings"

	"github.com/alecthomas/kong"
	"github.com/charmbracelet/log"
	"github.com/jrwynneiii/goestuner/config"
	"github.com/jrwynneiii/goestuner/datalink"
	"github.com/jrwynneiii/goestuner/demod"
	"github.com/jrwynneiii/goestuner/radio"
	"github.com/jrwynneiii/goestuner/tui"

	"github.com/knadh/koanf/parsers/hcl"
	"github.com/knadh/koanf/providers/env/v2"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/v2"
)

var cli struct {
	Verbose bool `help:"Prints debug output by default"`
	Profile bool `help:"Output a pprof profile"`
	Probe   struct {
	} `cmd:"" help:"List the available radios and SoapySDR configuration"`
	Tune struct {
	} `cmd:"" help:"Starts the TUI and connects to the SDR"`
}

var configFile = koanf.New(".")

func getConfigPath() string {
	paths := []string{"/etc/goestuner/config.hcl", "~/.config/goestuner/config.hcl", "./config.hcl"}
	for _, path := range paths {
		if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
			log.Infof("Found config file: %s", path)
			return path
		}
	}
	log.Info("Config file not found!")
	return ""
}

func main() {
	log.Info("Starting GOESWatcher")
	flags := kong.Parse(&cli)
	if cli.Verbose {
		log.SetLevel(log.DebugLevel)
	}

	if cli.Profile {
		prof, err := os.Create("./cpu.pprof")
		if err != nil {
			panic(err)
		}
		pprof.StartCPUProfile(prof)
		defer pprof.StopCPUProfile()
	}

	if err := configFile.Load(file.Provider(getConfigPath()), hcl.Parser(true)); err != nil {
		log.Errorf("Could not read config file: %v", err)
		log.Error("Attempting to use environment variables")
		configFile.Load(env.Provider("", env.Opt{
			Prefix: "GOESTUNER_",
			TransformFunc: func(k, v string) (string, any) {
				key := strings.ToLower(strings.TrimPrefix(k, "GOESTUNER_"))
				k = strings.Replace(key, "_", ".", 1)
				fmt.Printf("Found config env var: %s=%v\n", k, v)
				return k, v
			},
		}), nil)

	}

	switch flags.Command() {
	case "probe":
		radio.LogAllSoapySDRDevices()

	case "tune":
		rname := configFile.String("radio.driver")

		rdef := config.RadioConf{
			Address:     configFile.String("radio.address"),
			DeviceIndex: configFile.Int("radio.device_index"),
			Gain:        configFile.Int("radio.gain"),
			Frequency:   configFile.Float64("radio.frequency"),
			SampleRate:  configFile.Float64("radio.sample_rate"),
			SampleType:  configFile.String("radio.sample_type"),
			Decimation:  configFile.String("radio.decimation"),
		}
		tuiDef := config.TuiConf{
			RefreshMs:       configFile.Int("tui.refresh_ms"),
			RsWarnPct:       configFile.Float64("tui.rs_threshold_warn_pct"),
			RsCritPct:       configFile.Float64("tui.rs_threshold_crit_pct"),
			VitWarnPct:      configFile.Float64("tui.vit_threshold_warn_pct"),
			VitCritPct:      configFile.Float64("tui.vit_threshold_crit_pct"),
			EnableLogOutput: configFile.Bool("tui.enable_log_output"),
		}
		xritChunkSize := uint(configFile.Int("xrit.chunk_size"))
		xritDoFFT := configFile.Bool("xrit.do_fft")

		log.Debugf("Found radio definition for %s: %##v", rname, rdef)
		log.Debug("Starting init of SDR")
		switch rdef.SampleType {
		case "complex64":
			decoder := datalink.New(xritChunkSize, configFile)
			demodulator := demod.New(radio.CF32, float32(rdef.SampleRate), xritChunkSize, configFile, &decoder.SymbolsInput)
			r := radio.New[complex64](rdef, rname, radio.CF32, xritChunkSize, &demodulator.SampleInput)
			r.Connect()

			go r.Start()
			go demodulator.Start()
			go decoder.Start()
			defer demodulator.Close()
			defer decoder.Close()
			defer r.Destroy()

			tui.StartUI(decoder, demodulator, r, xritDoFFT, tuiDef)
		default:
			log.Fatalf("Unsupported sample_type defined for radio %s\n Supported sample types are: [CF32]", rname)
		}
	default:
		log.Info("Command not recognized")
	}
}
