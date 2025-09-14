package config

type RadioConf struct {
	Address     string  `koanf:"address"`
	DeviceIndex int     `koanf:"device_index"`
	Gain        int     `koanf:"gain"`
	Frequency   float64 `koanf:"frequency"`
	SampleRate  float64 `koanf:"sample_rate"`
	SampleType  string  `koanf:"sample_type"`
	Decimation  string  `koanf:"decimation"`
}

type AGCConf struct {
	Rate      float32 `koanf:"rate"`
	Reference float32 `koanf:"reference"`
	Gain      float32 `koanf:"gain"`
	MaxGain   float32 `koanf:"max_gain"`
}

type ClockRecoveryConf struct {
	Mu         float32 `koanf:"mu"`
	Alpha      float32 `koanf:"alpha"`
	OmegaLimit float32 `koanf:"omega_limit"`
}

type XRITConf struct {
	SymbolRate             float64 `koanf:"symbol_rate"`
	RRCAlpha               float64 `koanf:"rrc_alpha"`
	RRCTaps                int     `koanf:"rrc_taps"`
	LowPassTransitionWidth float64 `koanf:"lowpass_transition_width"`
	PLLAlpha               float32 `koanf:"pll_alpha"`
	Decimation             int     `koanf:"decimation_factor"`
	ChunkSize              uint    `koanf:"chunk_size"`
}

type XRITFrameConf struct {
	FrameSize     int `koanf:"frame_size"`
	LastFrameSize int `koanf:"last_frame_size"`
}

type ViterbiConf struct {
	MaxErrors int `koanf:"max_errors"`
}
