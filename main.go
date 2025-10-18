package main

import (
	"errors"
	"fmt"
	"os"
	"runtime/pprof"
	"strings"
	"time"

	"github.com/alecthomas/kong"
	"github.com/charmbracelet/log"
	"github.com/jrwynneiii/ccsds_tools"
	"github.com/jrwynneiii/ccsds_tools/layers/datalink"
	"github.com/jrwynneiii/ccsds_tools/layers/physical"
	"github.com/jrwynneiii/ccsds_tools/pipeline"
	"github.com/jrwynneiii/goestuner/config"
	"github.com/jrwynneiii/goestuner/radio"
	"github.com/jrwynneiii/goestuner/tui"
	"github.com/jrwynneiii/goestuner/types"

	"github.com/knadh/koanf/parsers/hcl"
	"github.com/knadh/koanf/providers/env/v2"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/v2"
	SatHelper "github.com/opensatelliteproject/libsathelper"
)

var cli struct {
	Verbose bool `help:"Prints debug output by default"`
	Profile bool `help:"Output a pprof profile"`
	Probe   struct {
	} `cmd:"" help:"List the available radios and SoapySDR configuration"`
	Tune struct {
	} `cmd:"" help:"Starts the TUI and connects to the SDR"`
	Config struct {
	} `cmd:"" help:"Opens the configuration file creator"`
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
	case "config":
		config.AutoConfig()
	case "probe":
		radio.LogAllSoapySDRDevices()

	case "tune":
		rname := configFile.String("radio.driver")

		rdef := types.RadioConf{
			Address:     configFile.String("radio.address"),
			DeviceIndex: configFile.Int("radio.device_index"),
			Gain:        configFile.Int("radio.gain"),
			Frequency:   configFile.Float64("radio.frequency"),
			SampleRate:  configFile.Float64("radio.sample_rate"),
			SampleType:  configFile.String("radio.sample_type"),
			Decimation:  configFile.String("radio.decimation"),
		}
		tuiDef := types.TuiConf{
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
		log.Debugf("Starting CCSDS pipeline")

		pipeline := pipeline.New(configFile)
		pipeline.Register(ccsds_tools.PhysicalLayer)
		pipeline.Register(ccsds_tools.DataLinkLayer)
		framesOut := pipeline.Layers[ccsds_tools.DataLinkLayer].GetOutput().(*chan []byte)
		samplesIn := pipeline.Layers[ccsds_tools.PhysicalLayer].GetInput().(*chan []complex64)

		r := radio.New(rdef, rname, xritChunkSize, samplesIn)
		r.Connect()

		log.Debug("Starting init of SDR")
		go r.Start()
		pipeline.Start()

		defer pipeline.Destroy()
		defer r.Destroy()

		go func() {
			for {
				select {
				case frame := <-*framesOut:
					vcid := frame[1] & 0x3F
					counter := uint(frame[2])
					counter = SatHelper.ToolsSwapEndianess(counter)
					counter &= 0xFFFFFF00
					counter >>= 8
					log.Infof("Got frame: vcid: %d (%s) object number: %d", int(vcid), datalink.VCIDs[int(vcid)], counter)
				default:
					time.Sleep(50 * time.Millisecond)
				}
			}
		}()

		tui.StartUI(pipeline.Layers[ccsds_tools.DataLinkLayer].(*datalink.Decoder), pipeline.Layers[ccsds_tools.PhysicalLayer].(*physical.Demodulator), r, xritDoFFT, tuiDef)
	default:
		log.Info("Command not recognized")
	}
}
