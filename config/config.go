package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/charmbracelet/log"
	"github.com/hashicorp/hcl/v2/gohcl"
	"github.com/hashicorp/hcl/v2/hclwrite"
	"github.com/jrwynneiii/goestuner/radio"
)

func AutoConfig(path string) {
	log.Info("Getting SDR Info (This may take a minute...)")
	if subsys, err := radio.InitSoapySDR(); err == nil {
		StartConfigTUI(subsys, path)
	} else {
		panic(err)
	}
}

func GenerateConfigFile(path string, driver string, device string, address string, port string, gain string, freq string, srate string) {
	dir := filepath.Dir(path)
	log.Infof("Ensuring directory %s exists", dir)

	// Make config dir if not exists
	if err := os.MkdirAll(dir, os.ModePerm); err != nil {
		log.Errorf("Error creating config directory: %w", err)
	}

	gainInt, _ := strconv.Atoi(gain)
	freqInt, _ := strconv.Atoi(freq)
	srateInt, _ := strconv.Atoi(srate)

	defaultVals := ConfigFile{
		Agc{
			Rate:      0.01,
			Reference: 0.5,
			Gain:      1.0,
			MaxGain:   4000,
		},
		Clockrecovery{
			Mu:         0.5,
			Alpha:      0.0037,
			OmegaLimit: 0.005,
		},
		Radio{
			Decimation: 1,
			Driver:     driver,
			Frequency:  freqInt,
			Gain:       gainInt,
			Name:       device,
			SampleRate: srateInt,
		},
		Tui{
			EnableLogOutput:     true,
			RefreshMs:           500,
			RsThresholdCritPct:  5,
			RsThresholdWarnPct:  2,
			VitThresholdCritPct: 5,
			VitThresholdWarnPct: 3,
		},
		Viterbi{
			MaxErrors: 500,
		},
		Xrit{
			ChunkSize:              66560,
			DecimationFactor:       1,
			DoFft:                  true,
			LowpassTransitionWidth: 200000,
			PllAlpha:               0.001,
			RrcAlpha:               0.3,
			RrcTaps:                31,
			SymbolRate:             927000,
		},
		Xritframe{
			FrameSize:     1024,
			LastFrameSize: 8,
		},
	}

	if len(address) > 0 && len(port) > 0 {
		defaultVals.Radio.Address = fmt.Sprintf("%s:%s", address, port)
	}

	log.Info("Generated config...")

	f := hclwrite.NewEmptyFile()
	gohcl.EncodeIntoBody(&defaultVals, f.Body())
	fmt.Printf("%s", f.Bytes())

	if err := os.WriteFile(path, f.Bytes(), os.FileMode(0644)); err != nil {
		log.Fatal(err)
	}

}
